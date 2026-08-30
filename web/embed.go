// Package web carries the compiled Vue frontend inside the binary.
//
// The build output in dist/ is produced by `npm run build` and embedded here,
// so a released Cairn is one file with no assets to deploy alongside it.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Assets returns the built frontend rooted at dist/.
func Assets() (fs.FS, error) { return fs.Sub(dist, "dist") }

// Built reports whether a frontend was compiled into this binary. A Go-only
// build (a clean checkout, `go build` with no `npm run build` first) embeds an
// empty directory rather than failing to compile.
func Built() bool {
	f, err := dist.Open("dist/index.html")
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// Handler serves the frontend, falling back to index.html for any path that is
// not a file, because the routes below / are resolved by the Vue router in the
// browser rather than here.
func Handler() http.Handler {
	assets, err := Assets()
	if err != nil || !Built() {
		return http.HandlerFunc(notBuilt)
	}
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}

		if f, err := assets.Open(name); err == nil {
			f.Close()
			// Vite fingerprints everything under assets/, so those are safe to
			// cache forever. index.html never is: it names the fingerprints.
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			files.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			notBuilt(w, r)
			return
		}
		w.Write(index)
	})
}

func notBuilt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("cairn: no frontend in this binary\n\n" +
		"The Vue app was not built before `go build`. Run:\n\n" +
		"    make build\n\n" +
		"or, to do it by hand:\n\n" +
		"    cd web && npm install && npm run build\n" +
		"    go build ./cmd/cairn\n"))
}
