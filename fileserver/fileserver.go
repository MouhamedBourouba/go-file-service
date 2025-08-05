package fileserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileServer struct {
	dataDir     string
	readOnly    bool
	allowDelete bool
	maxFileSize int64
	logger      *log.Logger
}

type Option func(*FileServer)

type FileInfo struct {
	Name     string `json:"name" example:"example.txt"`
	IsDir    bool   `json:"isDir" example:"false"`
	Size     int64  `json:"size" example:"1024"`
	ModTime  string `json:"modTime" example:"2024-01-01T12:00:00Z"`
	Path     string `json:"path" example:"/folder/example.txt"`
	MimeType string `json:"mimeType,omitempty" example:"text/plain"`
}

type DirectoryResponse struct {
	Path      string     `json:"path" example:"/folder"`
	Files     []FileInfo `json:"files"`
	TotalSize int64      `json:"totalSize" example:"10240"`
	Count     int        `json:"count" example:"5"`
}

type ErrorResponse struct {
	Error     string `json:"error" example:"File not found"`
	Message   string `json:"message" example:"The requested file does not exist"`
	Timestamp string `json:"timestamp" example:"2024-01-01T12:00:00Z"`
	Path      string `json:"path,omitempty" example:"/invalid/path"`
}

type UploadResponse struct {
	Message   string `json:"message" example:"File created successfully"`
	Path      string `json:"path" example:"/folder/example.txt"`
	Size      int64  `json:"size" example:"1024"`
	Timestamp string `json:"timestamp" example:"2024-01-01T12:00:00Z"`
}

func WithDataDir(dataDir string) Option {
	return func(fs *FileServer) {
		cleanedPath := path.Clean(dataDir)
		if stats, err := os.Stat(cleanedPath); err != nil {
			log.Fatalf("Cannot access data directory '%s': %v", cleanedPath, err)
		} else if !stats.IsDir() {
			log.Fatalf("Data directory path '%s' is not a directory", cleanedPath)
		}
		fs.dataDir = cleanedPath
	}
}

func WithReadOnly(readOnly bool) Option {
	return func(fs *FileServer) {
		fs.readOnly = readOnly
	}
}

func WithAllowDelete(allow bool) Option {
	return func(fs *FileServer) {
		fs.allowDelete = allow
	}
}

func WithMaxFileSize(size int64) Option {
	return func(fs *FileServer) {
		fs.maxFileSize = size
	}
}

func WithLogger(logger *log.Logger) Option {
	return func(fs *FileServer) {
		fs.logger = logger
	}
}

func New(options ...Option) *FileServer {
	fs := &FileServer{
		dataDir:     "./",
		readOnly:    false,
		allowDelete: true,
		maxFileSize: 100 * 1024 * 1024, // 100MB default
		logger:      log.Default(),
	}

	for _, option := range options {
		option(fs)
	}

	return fs
}

func (fs *FileServer) securePath(urlPath string) (string, error) {
	cleanedPath := filepath.Clean(urlPath)

	if strings.Contains(cleanedPath, "..") || strings.HasPrefix(cleanedPath, "/..") {
		return "", errors.New("path traversal not allowed")
	}

	cleanedPath = strings.TrimPrefix(cleanedPath, "/")
	fullPath := filepath.Join(fs.dataDir, cleanedPath)

	absDataDir, err := filepath.Abs(fs.dataDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve data directory: %w", err)
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve requested path: %w", err)
	}

	if !strings.HasPrefix(absFullPath, absDataDir) {
		return "", errors.New("path outside of allowed directory")
	}

	return fullPath, nil
}

func (fs *FileServer) logRequest(r *http.Request, status int, message string) {
	fs.logger.Printf("%s %s %d - %s", r.Method, r.URL.Path, status, message)
}

// @Summary Get file or directory
// @Description Get a file's content or list directory contents
// @Tags files
// @Param path path string true "File or directory path"
// @Param Accept header string false "Accept header" Enums(application/json,text/html)
// @Produce json,octet-stream,text/html
// @Success 200 {object} DirectoryResponse "Directory listing (JSON)"
// @Success 200 {file} file "File content"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 404 {object} ErrorResponse "File not found"
// @Router /{path} [get]
func (fs *FileServer) getRequest(w http.ResponseWriter, r *http.Request) {
	requestedFile, err := fs.securePath(r.URL.Path)
	if err != nil {
		fs.writeError(w, r, "Invalid path", http.StatusBadRequest, err.Error())
		return
	}

	stats, err := os.Stat(requestedFile)
	if err != nil {
		if os.IsNotExist(err) {
			fs.writeError(w, r, "File not found", http.StatusNotFound, fmt.Sprintf("'%s' does not exist", r.URL.Path))
			return
		}
		fs.writeError(w, r, "Cannot access file", http.StatusInternalServerError, err.Error())
		return
	}

	if stats.IsDir() {
		fs.serveDirectory(w, r, requestedFile)
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(stats.Size(), 10))
	w.Header().Set("Last-Modified", stats.ModTime().UTC().Format(http.TimeFormat))

	fs.logRequest(r, http.StatusOK, fmt.Sprintf("served file: %s (%d bytes)", r.URL.Path, stats.Size()))
	http.ServeFile(w, r, requestedFile)
}

// @Summary Upload or create file
// @Description Upload a new file or update existing file content
// @Tags files
// @Param path path string true "File path"
// @Param file body string true "File content"
// @Accept octet-stream,text/plain,multipart/form-data
// @Produce json
// @Success 200 {object} UploadResponse "File updated"
// @Success 201 {object} UploadResponse "File created"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 413 {object} ErrorResponse "File too large"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /files/{path} [put]
func (fs *FileServer) putRequest(w http.ResponseWriter, r *http.Request) {
	if fs.readOnly {
		fs.writeError(w, r, "Server is read-only", http.StatusForbidden, "Write operations are disabled")
		return
	}

	requestedFile, err := fs.securePath(r.URL.Path)
	if err != nil {
		fs.writeError(w, r, "Invalid path", http.StatusBadRequest, err.Error())
		return
	}

	if r.ContentLength > fs.maxFileSize {
		fs.writeError(w, r, "File too large", http.StatusRequestEntityTooLarge,
			fmt.Sprintf("File size %d exceeds maximum %d", r.ContentLength, fs.maxFileSize))
		return
	}

	dir := filepath.Dir(requestedFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fs.writeError(w, r, "Cannot create directory", http.StatusInternalServerError, err.Error())
		return
	}

	_, err = os.Stat(requestedFile)
	isNew := os.IsNotExist(err)

	file, err := os.Create(requestedFile)
	if err != nil {
		fs.writeError(w, r, "Cannot create file", http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()

	written, err := io.CopyN(file, r.Body, fs.maxFileSize+1)
	if err != nil && err != io.EOF {
		fs.writeError(w, r, "Cannot write file content", http.StatusInternalServerError, err.Error())
		return
	}

	if written > fs.maxFileSize {
		os.Remove(requestedFile)
		fs.writeError(w, r, "File too large", http.StatusRequestEntityTooLarge,
			fmt.Sprintf("File size exceeds maximum %d", fs.maxFileSize))
		return
	}

	status := http.StatusOK
	message := "File updated successfully"
	if isNew {
		status = http.StatusCreated
		message = "File created successfully"
	}

	response := UploadResponse{
		Message:   message,
		Path:      r.URL.Path,
		Size:      written,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	fs.logRequest(r, status, fmt.Sprintf("%s: %s (%d bytes)", message, r.URL.Path, written))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

// @Summary Delete file or directory
// @Description Delete a file or empty directory
// @Tags files
// @Param path path string true "File or directory path"
// @Param recursive query bool false "Delete directory recursively"
// @Produce json
// @Success 200 {object} map[string]string "Success message"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "File not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /files/{path} [delete]
func (fs *FileServer) deleteRequest(w http.ResponseWriter, r *http.Request) {
	if fs.readOnly {
		fs.writeError(w, r, "Server is read-only", http.StatusForbidden, "Delete operations are disabled")
		return
	}

	if !fs.allowDelete {
		fs.writeError(w, r, "Delete operations not allowed", http.StatusForbidden, "Delete operations are disabled by configuration")
		return
	}

	requestedFile, err := fs.securePath(r.URL.Path)
	if err != nil {
		fs.writeError(w, r, "Invalid path", http.StatusBadRequest, err.Error())
		return
	}

	stats, err := os.Stat(requestedFile)
	if err != nil {
		if os.IsNotExist(err) {
			fs.writeError(w, r, "File not found", http.StatusNotFound, fmt.Sprintf("'%s' does not exist", r.URL.Path))
			return
		}
		fs.writeError(w, r, "Cannot access file", http.StatusInternalServerError, err.Error())
		return
	}

	if stats.IsDir() {
		recursive := r.URL.Query().Get("recursive") == "true"
		if recursive {
			err = os.RemoveAll(requestedFile)
		} else {
			err = os.Remove(requestedFile)
		}
	} else {
		err = os.Remove(requestedFile)
	}

	if err != nil {
		fs.writeError(w, r, "Cannot delete", http.StatusInternalServerError, err.Error())
		return
	}

	fs.logRequest(r, http.StatusOK, fmt.Sprintf("deleted: %s", r.URL.Path))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "Successfully deleted",
		"path":      r.URL.Path,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (fs *FileServer) writeError(w http.ResponseWriter, r *http.Request, message string, statusCode int, details string) {
	fs.logRequest(r, statusCode, fmt.Sprintf("%s: %s", message, details))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := ErrorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Path:      r.URL.Path,
	}

	json.NewEncoder(w).Encode(errorResponse)
}

func (fs *FileServer) serveDirectory(w http.ResponseWriter, r *http.Request, dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		fs.writeError(w, r, "Cannot read directory", http.StatusInternalServerError, err.Error())
		return
	}

	var files []FileInfo
	var totalSize int64

	relativePath := strings.TrimPrefix(dirPath, fs.dataDir)
	if relativePath == "" {
		relativePath = "/"
	}

	for _, entry := range entries {

		info, err := entry.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(relativePath, entry.Name())
		if filepath.Separator != '/' {
			fullPath = strings.ReplaceAll(fullPath, string(filepath.Separator), "/")
		}

		fileInfo := FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
			Path:    fullPath,
		}

		if !entry.IsDir() {
			if mimeType := mime.TypeByExtension(filepath.Ext(entry.Name())); mimeType != "" {
				fileInfo.MimeType = mimeType
			}
			totalSize += info.Size()
		}

		files = append(files, fileInfo)
	}

	response := DirectoryResponse{
		Path:      relativePath,
		Files:     files,
		TotalSize: totalSize,
		Count:     len(files),
	}

	fs.logRequest(r, http.StatusOK, fmt.Sprintf("listed directory: %s (%d items)", relativePath, len(files)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (fs *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		fs.getRequest(w, r)
	case http.MethodPut:
		fs.putRequest(w, r)
	case http.MethodDelete:
		fs.deleteRequest(w, r)
	default:
		fs.writeError(w, r, "Method not allowed", http.StatusMethodNotAllowed,
			fmt.Sprintf("Method '%s' is not supported", r.Method))
	}
}
