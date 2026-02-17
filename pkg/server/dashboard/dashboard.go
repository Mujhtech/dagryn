package dashboard

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// Handler returns an HTTP handler that serves the dashboard static files.
// It handles SPA routing by serving index.html for any path that doesn't match a file.
func Handler() http.Handler {
	// Get the dist subdirectory
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Return a handler that always returns 500 if embed fails
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Dashboard not available", http.StatusInternalServerError)
		})
	}

	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		urlPath := path.Clean(r.URL.Path)
		if urlPath == "/" {
			urlPath = "/index.html"
		}

		// Try to open the file
		filePath := strings.TrimPrefix(urlPath, "/")
		file, err := dist.Open(filePath)
		if err != nil {
			// File not found - serve index.html for SPA routing
			serveIndexHTML(w, dist)
			return
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("Failed to close file: %v", err)
			}
		}()

		// File exists - serve it with appropriate content type
		fileServer.ServeHTTP(w, r)
	})
}

// serveIndexHTML serves the index.html file for SPA client-side routing
func serveIndexHTML(w http.ResponseWriter, dist fs.FS) {
	file, err := dist.Open("index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusNotFound)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Failed to close file: %v", err)
		}
	}()

	// Get file info for content length
	stat, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to stat index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, file); err != nil {
		http.Error(w, "Failed to copy index.html", http.StatusInternalServerError)
		return
	}
}
