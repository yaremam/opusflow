// Package httpserver builds the application's HTTP handler tree.
package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// New returns the application's root HTTP handler. When staticDir is
// non-empty, it also serves the built web app from that directory, falling
// back to its index.html for any unmatched GET so client-side routing keeps
// working after a refresh.
func New(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)

	if staticDir != "" {
		mux.Handle("GET /", spaHandler(staticDir))
	}

	return mux
}

type healthResponse struct {
	Status string `json:"status"`
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

func spaHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(staticDir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); err != nil {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
