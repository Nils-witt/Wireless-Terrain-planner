// Package web serves the compiled frontend, which is embedded into the
// binary.
//
// The frontend build (npm run build in frontend/) writes into dist/. Only
// dist/.gitkeep is committed, so the Go packages compile without a frontend
// build; Handler then reports that the frontend is missing.
package web

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the embedded frontend.
func Handler() (http.Handler, error) {
	files, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}
	return newHandler(files)
}

func newHandler(files fs.FS) (http.Handler, error) {
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return nil, errors.New("frontend not built: no index.html embedded (run npm run build in frontend/)")
	}
	fileServer := http.FileServerFS(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vite content-hashes everything under /assets/, so it never changes
		// at a given URL. index.html is not hashed and stays uncached. Errors
		// served by the file server drop this header again.
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
