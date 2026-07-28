package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
)

// maxArtworkUploadBytes caps a manually-uploaded photo/cover (TDR 007 AC-8)
// — small enough to keep a single request cheap, generous enough for any
// real photo/cover image.
const maxArtworkUploadBytes = 8 << 20 // 8 MiB

// readUploadedImage extracts the "image" multipart field, enforcing
// maxArtworkUploadBytes on the whole request body (not just the one field)
// via http.MaxBytesReader — the standard way to bound an upload's size
// before any of it is read into memory.
func readUploadedImage(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxArtworkUploadBytes)
	if err := r.ParseMultipartForm(maxArtworkUploadBytes); err != nil {
		return nil, errors.New("image exceeds the 8MB upload limit, or the request isn't a valid multipart form")
	}
	file, _, err := r.FormFile("image")
	if err != nil {
		return nil, errors.New(`an "image" multipart field is required`)
	}
	defer file.Close()
	return io.ReadAll(file)
}

// parseListOptions reads the sort/filter/pagination query params shared by
// the artists/albums/songs list endpoints. Missing or non-numeric values
// become zero values — Service.normalizeListOptions is what turns those
// into sane defaults, so this stays a dumb parse with no validation logic
// of its own.
func parseListOptions(r *http.Request) library.ListOptions {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	year, _ := strconv.Atoi(q.Get("year"))
	return library.ListOptions{
		Page:     page,
		PageSize: pageSize,
		Sort:     q.Get("sort"),
		Genre:    q.Get("genre"),
		Year:     year,
		Query:    q.Get("q"),
	}
}

func handleListArtists(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := svc.ListArtists(r.Context(), parseListOptions(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func handleGetArtist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid artist id", http.StatusBadRequest)
			return
		}
		artist, err := svc.GetArtist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, artist)
	}
}

func handleDeleteArtist(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid artist id", http.StatusBadRequest)
			return
		}
		deleteFiles, err := parseDeleteFiles(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := svc.DeleteArtist(r.Context(), id, deleteFiles); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMerge folds the entity at {id} — every album/track/photo (an
// artist) or track/cover (an album) it has — into the entity named by the
// request body's intoId (TDR 018), then removes {id}. Returns the
// survivor's fresh detail, since the page the request came from no longer
// exists. Shared implementation behind handleMergeArtist/handleMergeAlbum
// below — identical except which Service methods they close over.
func handleMerge[D any](entityLabel string, getDetail func(ctx context.Context, id int64) (D, error), merge func(ctx context.Context, id, intoID int64) error) http.HandlerFunc {
	type mergeRequest struct {
		IntoID int64 `json:"intoId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid "+entityLabel+" id", http.StatusBadRequest)
			return
		}
		var req mergeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if req.IntoID == 0 {
			http.Error(w, "intoId is required", http.StatusBadRequest)
			return
		}
		if err := merge(r.Context(), id, req.IntoID); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		target, err := getDetail(r.Context(), req.IntoID)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, target)
	}
}

func handleMergeArtist(svc *library.Service) http.HandlerFunc {
	return handleMerge("artist", svc.GetArtist, svc.MergeArtists)
}

// galleryRoutes configures the four generic gallery handlers below for one
// entity kind (D is ArtistDetail or AlbumDetail) — request parsing and
// response shape are identical for an artist's photos and an album's
// covers; these closures are the only thing that varies.
type galleryRoutes[D any] struct {
	entityLabel  string // "artist" | "album", for error messages
	imageIDParam string // "photoId" | "coverId"
	getDetail    func(ctx context.Context, id int64) (D, error)
	retryArt     func(ctx context.Context, id int64) error
	uploadArt    func(ctx context.Context, id int64, data []byte) (D, error)
	deleteImage  func(ctx context.Context, entityID, imageID int64, deleteFile bool) error
}

func artistGalleryRoutes(svc *library.Service) galleryRoutes[library.ArtistDetail] {
	return galleryRoutes[library.ArtistDetail]{
		entityLabel:  "artist",
		imageIDParam: "photoId",
		getDetail:    svc.GetArtist,
		retryArt:     svc.RetryArtistArt,
		uploadArt:    svc.UploadArtistArt,
		deleteImage:  svc.DeleteArtistPhoto,
	}
}

func albumGalleryRoutes(svc *library.Service) galleryRoutes[library.AlbumDetail] {
	return galleryRoutes[library.AlbumDetail]{
		entityLabel:  "album",
		imageIDParam: "coverId",
		getDetail:    svc.GetAlbum,
		retryArt:     svc.RetryAlbumArt,
		uploadArt:    svc.UploadAlbumArt,
		deleteImage:  svc.DeleteAlbumCover,
	}
}

// handleRetryArt queues a fresh art lookup for one entity (TDR 007) and
// wakes the background enrichment job immediately — the response carries
// the entity as of right now (status back to pending), so the frontend has
// something to poll against without an extra round trip.
func handleRetryArt[D any](cfg galleryRoutes[D]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid "+cfg.entityLabel+" id", http.StatusBadRequest)
			return
		}
		if err := cfg.retryArt(r.Context(), id); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := cfg.getDetail(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusAccepted, detail)
	}
}

// handleUploadArt saves a manually-uploaded photo/cover (TDR 007), bypassing
// MusicBrainz/Cover Art Archive entirely — synchronous, no queueing/polling:
// the response already reflects the new image.
func handleUploadArt[D any](cfg galleryRoutes[D]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid "+cfg.entityLabel+" id", http.StatusBadRequest)
			return
		}
		data, err := readUploadedImage(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		detail, err := cfg.uploadArt(r.Context(), id, data)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

// parseDeleteFiles requires an explicit deleteFiles=true|false query
// parameter on every removal request — AC-13 rules out a silent default
// either way, so a missing or malformed value is a client error, not a
// fallback to "keep" or "delete".
func parseDeleteFiles(r *http.Request) (bool, error) {
	v := r.URL.Query().Get("deleteFiles")
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New(`deleteFiles query parameter must be "true" or "false"`)
	}
}

// parseDeleteFile is parseDeleteFiles for the single-image gallery removal
// endpoints (TDR 014 AC-4) — same required-explicit-choice rule, just
// singular ("deleteFile") since exactly one photo/cover is ever removed
// per call, unlike DeleteArtist/DeleteAlbum's whole-track-list removal.
func parseDeleteFile(r *http.Request) (bool, error) {
	v := r.URL.Query().Get("deleteFile")
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, errors.New(`deleteFile query parameter must be "true" or "false"`)
	}
}

// handleSetGalleryFlag marks one image in an entity's gallery via set —
// either a Store's SetXPrimaryY (the one shown in list views and grid
// tiles, AC-2) or SetXBannerY (the detail page header's banner, TDR 016);
// both are independent flags with an identical request/response shape.
// Returns the entity's full detail (gallery included) so the frontend can
// re-render without a second round trip.
func handleSetGalleryFlag[D any](cfg galleryRoutes[D], set func(ctx context.Context, entityID, imageID int64) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid "+cfg.entityLabel+" id", http.StatusBadRequest)
			return
		}
		imageID, err := strconv.ParseInt(r.PathValue(cfg.imageIDParam), 10, 64)
		if err != nil {
			http.Error(w, "invalid image id", http.StatusBadRequest)
			return
		}
		if err := set(r.Context(), entityID, imageID); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := cfg.getDetail(r.Context(), entityID)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

// handleDeleteGalleryImage removes one image from an entity's gallery
// (AC-4), optionally also deleting its file from disk per the required
// deleteFile query param (see parseDeleteFile).
func handleDeleteGalleryImage[D any](cfg galleryRoutes[D]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid "+cfg.entityLabel+" id", http.StatusBadRequest)
			return
		}
		imageID, err := strconv.ParseInt(r.PathValue(cfg.imageIDParam), 10, 64)
		if err != nil {
			http.Error(w, "invalid image id", http.StatusBadRequest)
			return
		}
		deleteFile, err := parseDeleteFile(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := cfg.deleteImage(r.Context(), entityID, imageID, deleteFile); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		detail, err := cfg.getDetail(r.Context(), entityID)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

func handleListAlbums(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := svc.ListAlbums(r.Context(), parseListOptions(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

func handleGetAlbum(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid album id", http.StatusBadRequest)
			return
		}
		album, err := svc.GetAlbum(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, album)
	}
}

func handleDeleteAlbum(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid album id", http.StatusBadRequest)
			return
		}
		deleteFiles, err := parseDeleteFiles(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := svc.DeleteAlbum(r.Context(), id, deleteFiles); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleMergeAlbum is handleMergeArtist's album counterpart.
func handleMergeAlbum(svc *library.Service) http.HandlerFunc {
	return handleMerge("album", svc.GetAlbum, svc.MergeAlbums)
}

func handleListSongs(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := svc.ListSongs(r.Context(), parseListOptions(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, page)
	}
}

// audioContentType maps a track's extension to the MIME type the
// in-browser player (TDR 015) expects — set explicitly rather than
// relying on Go's default mime package, which doesn't reliably know
// several of these.
func audioContentType(format string) string {
	switch format {
	case "mp3":
		return "audio/mpeg"
	case "flac":
		return "audio/flac"
	case "m4a":
		return "audio/mp4"
	case "ogg":
		return "audio/ogg"
	case "wv":
		return "audio/x-wavpack"
	default:
		return ""
	}
}

// handleStreamSong serves a track's audio bytes (TDR 015) — http.
// ServeContent handles HTTP range requests (206 Partial Content),
// conditional requests, and content sniffing against the opened file
// directly, so seeking works without any hand-rolled range-parsing here.
func handleStreamSong(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid song id", http.StatusBadRequest)
			return
		}

		path, err := svc.GetSongPath(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}

		f, err := os.Open(path)
		if err != nil {
			http.Error(w, "audio file not found on disk", http.StatusNotFound)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.Error(w, "reading audio file", http.StatusInternalServerError)
			return
		}

		if ct := audioContentType(library.TrackFormat(path)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		http.ServeContent(w, r, filepath.Base(path), info.ModTime(), f)
	}
}
