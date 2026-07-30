// Package httpserver builds the application's HTTP handler tree.
package httpserver

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yaremam/opusflow/backend/internal/auth"
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
//
// authStore backs /api/tokens (device pairing) and last-used tracking —
// nothing is gated (TDR 024, docs/tdr/024_drop_web_auth_gate_design.md):
// a Bearer token on an /api/ request just gets its usage recorded via
// auth.TrackTokenUsage if it matches a real row, and every request
// succeeds regardless. A nil authStore just skips both the /api/tokens
// routes and the tracking, for tests that aren't exercising auth.
func New(staticDir, artworkDir, version, buildDate string, svc *library.Service, authStore *auth.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handleHealth())
	mux.HandleFunc("GET /api/health", handleHealth())
	mux.HandleFunc("GET /api/about", handleAbout(version, buildDate))
	mux.HandleFunc("GET /api/config", handleConfig(svc))
	if authStore != nil {
		tokens := auth.NewService(authStore)
		mux.HandleFunc("POST /api/tokens", handleCreateToken(tokens))
		mux.HandleFunc("GET /api/tokens", handleListTokens(tokens))
		mux.HandleFunc("DELETE /api/tokens/{id}", handleDeleteToken(tokens))
	}
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
	mux.HandleFunc("POST /api/library/artists/{id}/merge", handleMergeArtist(svc))
	mux.HandleFunc("POST /api/library/artists/{id}/art/retry", handleRetryArt(artistGalleryRoutes(svc)))
	mux.HandleFunc("POST /api/library/artists/{id}/art", handleUploadArt(artistGalleryRoutes(svc)))
	mux.HandleFunc("POST /api/library/artists/{id}/photos/{photoId}/primary", handleSetGalleryFlag(artistGalleryRoutes(svc), svc.SetArtistPrimaryPhoto))
	mux.HandleFunc("POST /api/library/artists/{id}/photos/{photoId}/banner", handleSetGalleryFlag(artistGalleryRoutes(svc), svc.SetArtistBannerPhoto))
	mux.HandleFunc("DELETE /api/library/artists/{id}/photos/{photoId}", handleDeleteGalleryImage(artistGalleryRoutes(svc)))
	mux.HandleFunc("GET /api/library/albums", handleListAlbums(svc))
	mux.HandleFunc("GET /api/library/albums/{id}", handleGetAlbum(svc))
	mux.HandleFunc("DELETE /api/library/albums/{id}", handleDeleteAlbum(svc))
	mux.HandleFunc("POST /api/library/albums/{id}/merge", handleMergeAlbum(svc))
	mux.HandleFunc("POST /api/library/albums/{id}/art/retry", handleRetryArt(albumGalleryRoutes(svc)))
	mux.HandleFunc("POST /api/library/albums/{id}/art", handleUploadArt(albumGalleryRoutes(svc)))
	mux.HandleFunc("POST /api/library/albums/{id}/covers/{coverId}/primary", handleSetGalleryFlag(albumGalleryRoutes(svc), svc.SetAlbumPrimaryCover))
	mux.HandleFunc("POST /api/library/albums/{id}/covers/{coverId}/banner", handleSetGalleryFlag(albumGalleryRoutes(svc), svc.SetAlbumBannerCover))
	mux.HandleFunc("DELETE /api/library/albums/{id}/covers/{coverId}", handleDeleteGalleryImage(albumGalleryRoutes(svc)))
	mux.HandleFunc("GET /api/library/songs", handleListSongs(svc))
	mux.HandleFunc("GET /api/library/songs/{id}/stream", handleStreamSong(svc))
	mux.HandleFunc("GET /api/library/songs/{id}/playlists", handleListPlaylistsContainingTrack(svc))
	mux.HandleFunc("GET /api/playlists", handleListPlaylists(svc))
	mux.HandleFunc("POST /api/playlists", handleCreatePlaylist(svc))
	mux.HandleFunc("GET /api/playlists/{id}", handleGetPlaylist(svc))
	mux.HandleFunc("PATCH /api/playlists/{id}", handleRenamePlaylist(svc))
	mux.HandleFunc("DELETE /api/playlists/{id}", handleDeletePlaylist(svc))
	mux.HandleFunc("POST /api/playlists/{id}/tracks", handleAddTrackToPlaylist(svc))
	mux.HandleFunc("DELETE /api/playlists/{id}/tracks/{playlistTrackId}", handleRemovePlaylistTrack(svc))
	mux.HandleFunc("PATCH /api/playlists/{id}/tracks/reorder", handleReorderPlaylistTracks(svc))
	mux.HandleFunc("GET /api/metadata/artists", handleSearchArtists(svc))
	mux.HandleFunc("GET /api/metadata/artists/{mbid}/release-groups", handleArtistReleaseGroups(svc))
	mux.HandleFunc("GET /api/metadata/release-groups/{mbid}/releases", handleReleaseGroupReleases(svc))
	mux.HandleFunc("GET /api/metadata/releases/{mbid}/tracks", handleReleaseTracks(svc))

	if artworkDir != "" {
		mux.Handle("GET /artwork/", http.StripPrefix("/artwork/", http.FileServer(http.Dir(artworkDir))))
	}

	if staticDir != "" {
		mux.Handle("GET /", spaHandler(staticDir))
	}

	if authStore == nil {
		return mux
	}
	return trackTokenUsageUnderAPIPrefix(authStore, mux)
}

// trackTokenUsageUnderAPIPrefix applies auth.TrackTokenUsage only to
// requests under /api/ — no request is ever blocked by it (TDR 024); it
// only records that a token was used when one's present and valid.
func trackTokenUsageUnderAPIPrefix(store *auth.Store, next http.Handler) http.Handler {
	tracked := auth.TrackTokenUsage(store)(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			tracked.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
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
