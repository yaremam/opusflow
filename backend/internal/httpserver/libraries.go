package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
)

func handleListLibraries(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libs, err := svc.ListLibraries(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, libs)
	}
}

func handleCreateLibrary(svc *library.Service) http.HandlerFunc {
	type createRequest struct {
		Name     string `json:"name"`
		RootPath string `json:"rootPath"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if req.RootPath == "" {
			http.Error(w, "rootPath is required", http.StatusBadRequest)
			return
		}

		lib, err := svc.CreateLibrary(r.Context(), req.Name, req.RootPath)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusCreated, lib)
	}
}

func handleDeleteLibrary(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid library id", http.StatusBadRequest)
			return
		}
		deleteFiles, err := parseDeleteFiles(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := svc.DeleteLibrary(r.Context(), id, deleteFiles); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
