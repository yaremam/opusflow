// Package httpserver builds the application's HTTP handler tree.
package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/yaremam/opusflow/backend/internal/library"
)

// New returns the application's root HTTP handler. When staticDir is
// non-empty, it also serves the built web app from that directory, falling
// back to its index.html for any unmatched GET so client-side routing keeps
// working after a refresh. When artworkDir is non-empty, fetched/extracted
// artist photos and album covers (TDR 003) are served from it under
// /artwork/. svc backs the library endpoints.
func New(staticDir, artworkDir string, svc *library.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/library/roots", handleLibraryRoots(svc))
	mux.HandleFunc("GET /api/library/browse", handleLibraryBrowse(svc))
	mux.HandleFunc("GET /api/library/directories", handleListDirectories(svc))
	mux.HandleFunc("POST /api/library/directories", handleAddDirectory(svc))
	mux.HandleFunc("DELETE /api/library/directories/{id}", handleRemoveDirectory(svc))
	mux.HandleFunc("GET /api/library/artists", handleListArtists(svc))
	mux.HandleFunc("GET /api/library/artists/{id}", handleGetArtist(svc))
	mux.HandleFunc("GET /api/library/albums", handleListAlbums(svc))
	mux.HandleFunc("GET /api/library/albums/{id}", handleGetAlbum(svc))
	mux.HandleFunc("GET /api/library/songs", handleListSongs(svc))

	if artworkDir != "" {
		mux.Handle("GET /artwork/", http.StripPrefix("/artwork/", http.FileServer(http.Dir(artworkDir))))
	}

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
