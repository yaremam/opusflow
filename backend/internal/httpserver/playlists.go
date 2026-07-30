package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
)

func handleCreatePlaylist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		pl, err := svc.CreatePlaylist(r.Context(), req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, pl)
	}
}

func handleListPlaylists(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := svc.ListPlaylists(r.Context(), parseListOptions(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func handleGetPlaylist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		detail, err := svc.GetPlaylist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func handleRenamePlaylist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		if err := svc.RenamePlaylist(r.Context(), id, req.Name); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := svc.GetPlaylist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func handleDeletePlaylist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		if err := svc.DeletePlaylist(r.Context(), id); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAddTrackToPlaylist is "Add to playlist" (AC-5) — appends, no
// dedup (AC-6).
func handleAddTrackToPlaylist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		var req struct {
			TrackID int64 `json:"trackId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		pt, err := svc.AddTrackToPlaylist(r.Context(), id, req.TrackID)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusCreated, pt)
	}
}

// handleRemovePlaylistTrack removes one entry (addressed by its own row
// ID, not trackId — see PlaylistTrack's own doc comment for why) and
// returns the fresh detail, the same mutate-then-return-detail shape
// handleRenamePlaylist/handleReorderPlaylistTracks already use (and
// handleSetGalleryFlag/handleDeleteGalleryImage before them) — a removed
// track can shift CoverURLs if it was among the first four, so the
// client needs the recomputed detail back either way, not just a bare
// 204.
func handleRemovePlaylistTrack(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		playlistTrackID, err := strconv.ParseInt(r.PathValue("playlistTrackId"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist track id", http.StatusBadRequest)
			return
		}

		if err := svc.RemovePlaylistTrack(r.Context(), id, playlistTrackID); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := svc.GetPlaylist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func handleReorderPlaylistTracks(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid playlist id", http.StatusBadRequest)
			return
		}
		var req struct {
			PlaylistTrackID int64 `json:"playlistTrackId"`
			ToIndex         int   `json:"toIndex"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		if err := svc.ReorderPlaylistTracks(r.Context(), id, req.PlaylistTrackID, req.ToIndex); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := svc.GetPlaylist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

// handleListPlaylistsContainingTrack backs the "Add to playlist"
// picker's pre-checked state (AC-5).
func handleListPlaylistsContainingTrack(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		trackID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid track id", http.StatusBadRequest)
			return
		}
		playlists, err := svc.ListPlaylistsContainingTrack(r.Context(), trackID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, playlists)
	}
}
