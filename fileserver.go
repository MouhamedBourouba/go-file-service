package fileserver

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
)

type FileServer struct {
	root        string
	readOnly    bool
	allowDelete bool
}

type Option func(*FileServer)

func WithRoot(root string) Option {
	cleanedPath := path.Clean(root)
	stats, err := os.Stat(cleanedPath)

	if err != nil {
		log.Fatal("Can't open stat root path")
	} else if !stats.IsDir() {
		log.Fatal("root must be a diractory")
	}

	return func(fileserver *FileServer) {
		fileserver.root = cleanedPath
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
		root:        ".",
		readOnly:    false,
		allowDelete: true,
	}
	for _, option := range options {
		option(&fileserver)
	}
	return &fileserver
}

/*
 */
func parseGetRequest(r *http.Request) {
}

func parsePutRequest(r *http.Request) {
	println("put lol")
}

func parseDeleteRequest(r *http.Request) {
	println("delete lol")
}

func (*FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		parseGetRequest(r)
	case http.MethodPut:
		parsePutRequest(r)
	case http.MethodDelete:
		parseDeleteRequest(r)
	default:
		http.Error(w, fmt.Sprintf("Unsuppoted method '%s'", r.Method), http.StatusBadRequest)
	}
}

func FileSystemHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello im fs server bruhhhhh")
}
