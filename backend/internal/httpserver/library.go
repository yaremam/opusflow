package httpserver

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
)

func handleLibraryRoots(svc *library.Service) http.HandlerFunc {
	type rootResponse struct {
		Path string `json:"path"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		roots := svc.ListRoots()
		resp := make([]rootResponse, len(roots))
		for i, p := range roots {
			resp[i] = rootResponse{Path: p}
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleLibraryBrowse(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "path query parameter is required", http.StatusBadRequest)
			return
		}

		entries, err := svc.Browse(path)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if entries == nil {
			entries = []library.Entry{}
		}
		writeJSON(w, http.StatusOK, entries)
	}
}

func handleListDirectories(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dirs, err := svc.ListDirectories(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if dirs == nil {
			dirs = []library.Directory{}
		}
		writeJSON(w, http.StatusOK, dirs)
	}
}

func handleAddDirectory(svc *library.Service) http.HandlerFunc {
	type addRequest struct {
		Path string `json:"path"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req addRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Path == "" {
			http.Error(w, "path is required", http.StatusBadRequest)
			return
		}

		dir, err := svc.AddDirectory(r.Context(), req.Path)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}

		writeJSON(w, http.StatusCreated, dir)
	}
}

func handleRemoveDirectory(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid directory id", http.StatusBadRequest)
			return
		}

		if err := svc.RemoveDirectory(r.Context(), id); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// libraryErrorStatus maps a library package sentinel error to the HTTP
// status that best represents it to a client. Shared by every handler in
// this file so the mapping — and an unmapped error's fallback status —
// can't drift between them the way it did when each handler picked its own
// default (400 in one, 500 in another) for whatever it didn't recognize.
func libraryErrorStatus(err error) int {
	switch {
	case errors.Is(err, library.ErrOutsideRoots):
		return http.StatusForbidden
	case errors.Is(err, library.ErrNotADirectory):
		return http.StatusBadRequest
	case errors.Is(err, library.ErrDuplicateDirectory):
		return http.StatusConflict
	case errors.Is(err, library.ErrDirectoryNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrArtistNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrAlbumNotFound):
		return http.StatusNotFound
	case errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
