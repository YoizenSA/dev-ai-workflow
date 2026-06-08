package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed ui/*
var uiFS embed.FS

// uiHandler returns an http.Handler that serves the embedded Missions dashboard.
// It only serves the root path (/), /index.html, and /static/* paths.
// All other paths (especially /api/*) are left for the API routes.
func uiHandler() http.Handler {
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		panic("missions/web: failed to get ui sub-filesystem: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	staticServer := http.StripPrefix("/static/", fileServer)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "/index.html" {
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/static/") {
			staticServer.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
