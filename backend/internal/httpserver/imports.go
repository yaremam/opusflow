package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// planResponse is what every endpoint that hands back a plan returns: the
// plan itself (server-computed DestPath/Conflict included) plus the
// validation errors it currently has, if any — the reviewer needs both to
// render the review screen (AC-6 through AC-9).
type planResponse struct {
	Plan   organize.Plan              `json:"plan"`
	Errors []organize.ValidationError `json:"errors"`
}

func newPlanResponse(ctx context.Context, svc *library.Service, libraryID int64, plan organize.Plan) (planResponse, error) {
	errs, err := svc.ValidatePlan(ctx, libraryID, &plan)
	if err != nil {
		return planResponse{}, err
	}
	if errs == nil {
		errs = []organize.ValidationError{}
	}
	return planResponse{Plan: plan, Errors: errs}, nil
}

func handleImportBrowse(svc *library.Service) http.HandlerFunc {
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

func handleCreateFolder(svc *library.Service) http.HandlerFunc {
	type createFolderRequest struct {
		ParentPath string `json:"parentPath"`
		Name       string `json:"name"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req createFolderRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.ParentPath == "" {
			http.Error(w, "parentPath is required", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		entry, err := svc.CreateFolder(req.ParentPath, req.Name)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusCreated, entry)
	}
}

func handleBuildPlan(svc *library.Service) http.HandlerFunc {
	type buildRequest struct {
		LibraryID int64  `json:"libraryId"`
		SourceDir string `json:"sourceDir"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req buildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.SourceDir == "" {
			http.Error(w, "sourceDir is required", http.StatusBadRequest)
			return
		}
		if req.LibraryID == 0 {
			http.Error(w, "libraryId is required", http.StatusBadRequest)
			return
		}

		plan, err := svc.BuildPlan(r.Context(), req.LibraryID, req.SourceDir)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		resp, err := newPlanResponse(r.Context(), svc, req.LibraryID, plan)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleValidatePlan(svc *library.Service) http.HandlerFunc {
	type validateRequest struct {
		LibraryID int64         `json:"libraryId"`
		Plan      organize.Plan `json:"plan"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req validateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.LibraryID == 0 {
			http.Error(w, "libraryId is required", http.StatusBadRequest)
			return
		}

		resp, err := newPlanResponse(r.Context(), svc, req.LibraryID, req.Plan)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// maxUploadMemory bounds how much of a multipart upload's file parts are
// buffered in memory before spilling to disk — the request as a whole can
// be much larger (a full album's worth of audio files); this only caps how
// much of it sits in RAM at once while it's staged to a temp directory.
const maxUploadMemory = 32 << 20 // 32 MiB

func handleUploadImport(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}

		libraryID, err := strconv.ParseInt(r.FormValue("libraryId"), 10, 64)
		if err != nil {
			http.Error(w, "libraryId is required", http.StatusBadRequest)
			return
		}

		stagingDir, err := os.MkdirTemp("", "opusflow-upload-")
		if err != nil {
			http.Error(w, "staging upload", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(stagingDir)

		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			http.Error(w, "at least one file is required", http.StatusBadRequest)
			return
		}
		for _, fh := range files {
			if err := stageUploadedFile(stagingDir, fh); err != nil {
				http.Error(w, "staging "+fh.Filename+": "+err.Error(), http.StatusBadRequest)
				return
			}
		}

		plan, err := svc.BuildPlanFromStaged(r.Context(), libraryID, stagingDir)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		resp, err := newPlanResponse(r.Context(), svc, libraryID, plan)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// stageUploadedFile copies one uploaded part into dir, preserving the
// relative path the browser sent as the part's filename (webkitRelativePath
// for a folder upload) so BuildPlanFromStaged sees the same directory shape
// it would from a browsed server folder. The destination is computed by
// treating relPath as rooted — filepath.Clean collapses any ".." trying to
// climb above dir, the same defense http.Dir relies on — since relPath is
// client-controlled and must never be allowed to write outside the staging
// directory.
func stageUploadedFile(dir string, fh *multipart.FileHeader) error {
	rel := filepath.Clean(string(filepath.Separator) + fh.Filename)
	dest := filepath.Join(dir, rel)

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func handleConfirmImport(svc *library.Service) http.HandlerFunc {
	type confirmRequest struct {
		LibraryID         int64         `json:"libraryId"`
		SourceDescription string        `json:"sourceDescription"`
		Plan              organize.Plan `json:"plan"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req confirmRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.SourceDescription == "" {
			http.Error(w, "sourceDescription is required", http.StatusBadRequest)
			return
		}
		if req.LibraryID == 0 {
			http.Error(w, "libraryId is required", http.StatusBadRequest)
			return
		}

		imp, errs, err := svc.ConfirmImport(r.Context(), req.LibraryID, req.SourceDescription, req.Plan)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		if len(errs) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, struct {
				Errors []organize.ValidationError `json:"errors"`
			}{errs})
			return
		}
		writeJSON(w, http.StatusAccepted, imp)
	}
}

func handleListImports(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		imports, err := svc.ListImports(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, imports)
	}
}

func handleGetImport(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid import id", http.StatusBadRequest)
			return
		}

		imp, err := svc.GetImport(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, imp)
	}
}

// libraryErrorStatus maps a library package sentinel error to the HTTP
// status that best represents it to a client. Shared by every handler that
// touches library errors so the mapping — and an unmapped error's fallback
// status — can't drift between them the way it did when each handler picked
// its own default for whatever it didn't recognize.
func libraryErrorStatus(err error) int {
	switch {
	case errors.Is(err, library.ErrNotADirectory):
		return http.StatusBadRequest
	case errors.Is(err, library.ErrSourceInsideLibrary):
		return http.StatusBadRequest
	case errors.Is(err, library.ErrOutsideRoot):
		return http.StatusBadRequest
	case errors.Is(err, library.ErrInvalidFolderName):
		return http.StatusBadRequest
	case errors.Is(err, library.ErrLibraryNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrImportNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrArtistNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrAlbumNotFound):
		return http.StatusNotFound
	case errors.Is(err, library.ErrArtworkNotConfigured):
		return http.StatusServiceUnavailable
	case errors.Is(err, library.ErrMetadataSearchNotConfigured):
		return http.StatusServiceUnavailable
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
