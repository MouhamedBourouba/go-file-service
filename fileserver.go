package fileserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type FileServer struct {
	dataDir     string
	readOnly    bool
	allowDelete bool
}

type Option func(*FileServer)

func WithDataDir(dataDir string) Option {
	cleanedPath := path.Clean(dataDir)
	stats, err := os.Stat(cleanedPath)

	if err != nil {
		log.Fatal("Can't open stat root path")
	} else if !stats.IsDir() {
		log.Fatal("root must be a diractory")
	}

	return func(fileserver *FileServer) {
		fileserver.dataDir = cleanedPath
	}
}

func WithAllowDelete(allow bool) Option {
	return func(fileserver *FileServer) {
		fileserver.allowDelete = allow
	}
}

func WithReadOnly() Option {
	return func(fileserver *FileServer) {
		fileserver.readOnly = true
	}
}

func New(options ...Option) *FileServer {
	fileserver := FileServer{
		dataDir:     "./",
		readOnly:    false,
		allowDelete: true,
	}
	for _, option := range options {
		option(&fileserver)
	}
	return &fileserver
}

func (fs *FileServer) securePath(url string) (string, error) {
	cleanedPath := filepath.Clean(url)
	if strings.Contains(cleanedPath, "..") {
		return "", errors.New("Nice try lil bro")
	}
	joinedPath := filepath.Join(fs.dataDir, cleanedPath)
	return joinedPath, nil
}

func (fs *FileServer) getRequest(w http.ResponseWriter, r *http.Request) {
	requestedFile, err := fs.securePath(r.URL.Path)
	if err != nil {
		return
	}

	stats, err := os.Stat(requestedFile)
	if err != nil {
		http.Error(w, fmt.Sprintf("File requested dose not exist '%s'", requestedFile), http.StatusNotFound)
		return
	}

	if stats.IsDir() {
		println("yet implemented")
		return
	}

	http.ServeFile(w, r, requestedFile)
}

func PutRequest(w http.ResponseWriter, r *http.Request) {
	println("put lol")
}

func deleteRequest(w http.ResponseWriter, r *http.Request) {
	println("delete lol")
}

func (fs *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		fs.getRequest(w, r)
	case http.MethodPut:
		PutRequest(w, r)
	case http.MethodDelete:
		deleteRequest(w, r)
	default:
		http.Error(w, fmt.Sprintf("Unsuppoted method '%s'", r.Method), http.StatusBadRequest)
	}
}

func FileSystemHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello im fs server bruhhhhh")
}
