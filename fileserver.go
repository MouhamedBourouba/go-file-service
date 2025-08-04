package fileserver

import (
	"io"
	"log"
	"net/http"
	"os"
	"path"
)

/*
Features:
  - create files -> touch
  - update files
  - read files -> cat
  - read dir's -> ls
  - delete files/dir

GET
  - path is dir -> ls
  - path is file -> cat
  - path is invalid -> 400

PUT:
  - path is dir -> error
  - path is file -> replace
  - path is  -> replace
*/
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

func (*FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("the request")
}

func FileSystemHandler(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, "hello im fs server bruhhhhh")
}
