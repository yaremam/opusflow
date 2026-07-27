package httpserver

import (
	"net/http"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// handleSearchArtists backs the "look up metadata" review-screen flow's
// first step (TDR 012): every MusicBrainz artist matching q, for the user
// to pick the right one from before browsing their albums.
func handleSearchArtists(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, "q query parameter is required", http.StatusBadRequest)
			return
		}
		matches, err := svc.SearchArtists(r.Context(), q)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if matches == nil {
			matches = []enrich.ArtistMatch{}
		}
		writeJSON(w, http.StatusOK, matches)
	}
}

// handleArtistReleaseGroups lists every album MusicBrainz has on record
// for the artist identified by the {mbid} path value.
func handleArtistReleaseGroups(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		groups, err := svc.ArtistReleaseGroups(r.Context(), r.PathValue("mbid"))
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if groups == nil {
			groups = []enrich.ReleaseGroupMatch{}
		}
		writeJSON(w, http.StatusOK, groups)
	}
}

// handleReleaseGroupReleases lists every specific release/edition under
// the release-group identified by the {mbid} path value — the level
// track listings actually live at.
func handleReleaseGroupReleases(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		releases, err := svc.ReleaseGroupReleases(r.Context(), r.PathValue("mbid"))
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if releases == nil {
			releases = []enrich.ReleaseMatch{}
		}
		writeJSON(w, http.StatusOK, releases)
	}
}

// handleReleaseTracks fetches the full track listing for the release
// identified by the {mbid} path value.
func handleReleaseTracks(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tracks, err := svc.ReleaseTracks(r.Context(), r.PathValue("mbid"))
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if tracks == nil {
			tracks = []enrich.Track{}
		}
		writeJSON(w, http.StatusOK, tracks)
	}
}
