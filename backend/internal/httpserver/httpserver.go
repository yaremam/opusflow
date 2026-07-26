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
// /artwork/. revision (TDR 004's GIT_SHA) is reported by /health so a
// deployed instance can be identified; empty is a normal value (unset in
// local `go run`, not an error). svc backs the library endpoints.
func New(staticDir, artworkDir, revision string, svc *library.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth(revision))
	mux.HandleFunc("GET /api/libraries", handleListLibraries(svc))
	mux.HandleFunc("POST /api/libraries", handleCreateLibrary(svc))
	mux.HandleFunc("DELETE /api/libraries/{id}", handleDeleteLibrary(svc))
	mux.HandleFunc("GET /api/imports/browse", handleImportBrowse(svc))
	mux.HandleFunc("POST /api/imports/plan", handleBuildPlan(svc))
	mux.HandleFunc("POST /api/imports/plan/validate", handleValidatePlan(svc))
	mux.HandleFunc("POST /api/imports/upload", handleUploadImport(svc))
	mux.HandleFunc("POST /api/imports", handleConfirmImport(svc))
	mux.HandleFunc("GET /api/imports", handleListImports(svc))
	mux.HandleFunc("GET /api/imports/{id}", handleGetImport(svc))
	mux.HandleFunc("GET /api/library/artists", handleListArtists(svc))
	mux.HandleFunc("GET /api/library/artists/{id}", handleGetArtist(svc))
	mux.HandleFunc("DELETE /api/library/artists/{id}", handleDeleteArtist(svc))
	mux.HandleFunc("GET /api/library/albums", handleListAlbums(svc))
	mux.HandleFunc("GET /api/library/albums/{id}", handleGetAlbum(svc))
	mux.HandleFunc("DELETE /api/library/albums/{id}", handleDeleteAlbum(svc))
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
	Status   string `json:"status"`
	Revision string `json:"revision,omitempty"`
}

func handleHealth(revision string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(healthResponse{Status: "ok", Revision: revision})
	}
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
