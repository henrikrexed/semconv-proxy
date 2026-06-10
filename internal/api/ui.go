package api

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed ui/*
var uiFiles embed.FS

// uiHandler serves the embedded single-page UI. Unknown non-API paths fall back
// to index.html so client-side deep links (which live in the URL hash) resolve.
func uiHandler() http.Handler {
	sub, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err) // embed path is a compile-time constant; cannot fail at runtime
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An unmatched /api/ request must not fall through to the SPA shell:
		// API clients expect JSON, so a mistyped or removed endpoint returns a
		// JSON 404 rather than a 200 index.html.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		} else {
			path = path[1:] // strip leading slash for fs lookup
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Unknown asset: serve the SPA shell.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
