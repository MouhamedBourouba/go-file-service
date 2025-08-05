package main

import (
	"net/http"

	_ "github.com/mouhamedBourouba/go-file-service/docs"
	httpSwagger "github.com/swaggo/http-swagger"

	fileserver "github.com/mouhamedBourouba/go-file-service/fileserver"
)

// @title File Server API
// @version 1.0
// @description This is a sample file server API for web IDE
// @license.name MIT
// @host localhost:8000
// @BasePath /
func main() {
	http.Handle("/swagger/", httpSwagger.WrapHandler)

	fs := fileserver.New()
	http.Handle("/", fs)

	http.HandleFunc("/health", healthCheck)

	println("Server starting on :8000")
	println("Swagger UI: http://localhost:8000/swagger/index.html")
	println("Files: http://localhost:8000/files/")

	http.ListenAndServe(":8000", nil)
}

// @Summary Health check
// @Description Check if the server is running
// @Tags health
// @Produce json
// @Success 200 {object} map[string]string
// @Router /health [get]
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status": "ok"}`))
}
