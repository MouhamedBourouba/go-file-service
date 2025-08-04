package main

import (
	"net/http"

	fileserver "github.com/mouhamedBourouba/go-file-service"
)

func main() {
	fs := fileserver.New()

	http.Handle("/", fs)
	http.ListenAndServe(":8000", nil)
}
