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
// /artwork/. version and buildDate (TDR 009) are reported by /api/about so a
// deployed instance can be identified from within the app; empty is a
// normal value (unset in local `go run`, not an error). svc backs the
// library endpoints.
func New(staticDir, artworkDir, version, buildDate string, svc *library.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth())
	mux.HandleFunc("GET /api/about", handleAbout(version, buildDate))
	mux.HandleFunc("GET /api/config", handleConfig(svc))
	mux.HandleFunc("GET /api/libraries", handleListLibraries(svc))
	mux.HandleFunc("POST /api/libraries", handleCreateLibrary(svc))
	mux.HandleFunc("DELETE /api/libraries/{id}", handleDeleteLibrary(svc))
	mux.HandleFunc("GET /api/imports/browse", handleImportBrowse(svc))
	mux.HandleFunc("POST /api/imports/browse/folders", handleCreateFolder(svc))
	mux.HandleFunc("POST /api/imports/plan", handleBuildPlan(svc))
	mux.HandleFunc("POST /api/imports/plan/validate", handleValidatePlan(svc))
	mux.HandleFunc("POST /api/imports/upload", handleUploadImport(svc))
	mux.HandleFunc("POST /api/imports", handleConfirmImport(svc))
	mux.HandleFunc("GET /api/imports", handleListImports(svc))
	mux.HandleFunc("GET /api/imports/{id}", handleGetImport(svc))
	mux.HandleFunc("GET /api/library/artists", handleListArtists(svc))
	mux.HandleFunc("GET /api/library/artists/{id}", handleGetArtist(svc))
	mux.HandleFunc("DELETE /api/library/artists/{id}", handleDeleteArtist(svc))
	mux.HandleFunc("POST /api/library/artists/{id}/art/retry", handleRetryArtistArt(svc))
	mux.HandleFunc("POST /api/library/artists/{id}/art", handleUploadArtistArt(svc))
	mux.HandleFunc("GET /api/library/albums", handleListAlbums(svc))
	mux.HandleFunc("GET /api/library/albums/{id}", handleGetAlbum(svc))
	mux.HandleFunc("DELETE /api/library/albums/{id}", handleDeleteAlbum(svc))
	mux.HandleFunc("POST /api/library/albums/{id}/art/retry", handleRetryAlbumArt(svc))
	mux.HandleFunc("POST /api/library/albums/{id}/art", handleUploadAlbumArt(svc))
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

func handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	}
}

// aboutResponse identifies the running build for the About page (TDR 009):
// version is `git describe --tags --always` at image build time (e.g.
// "v0.1.0-4-gabc123f", or bare "v0.1.0" exactly on a tag), buildDate is the
// image's UTC build timestamp. Both are "dev"/empty for a plain `go run`
// with nothing stamped in.
type aboutResponse struct {
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
}

func handleAbout(version, buildDate string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, aboutResponse{Version: version, BuildDate: buildDate})
	}
}

// configResponse is frontend-relevant server configuration — currently just
// the browse root, so SourceFolderPicker knows where to start browsing
// instead of defaulting to "/" and immediately hitting ErrOutsideRoot.
type configResponse struct {
	DataDir string `json:"dataDir"`
}

func handleConfig(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, configResponse{DataDir: svc.BrowseRoot()})
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
