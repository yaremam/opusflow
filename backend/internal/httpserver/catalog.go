package httpserver

import (
	"errors"
	"io"
	"net/http"
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

// handleRetryArtistArt queues a fresh art lookup for one artist (TDR 007)
// and wakes the background enrichment job immediately — the response
// carries the artist as of right now (status back to pending), so the
// frontend has something to poll GetArtist against without an extra round
// trip.
func handleRetryArtistArt(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid artist id", http.StatusBadRequest)
			return
		}
		if err := svc.RetryArtistArt(r.Context(), id); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		artist, err := svc.GetArtist(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusAccepted, artist)
	}
}

// handleRetryAlbumArt is handleRetryArtistArt's album counterpart.
func handleRetryAlbumArt(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid album id", http.StatusBadRequest)
			return
		}
		if err := svc.RetryAlbumArt(r.Context(), id); err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		album, err := svc.GetAlbum(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusAccepted, album)
	}
}

// handleUploadArtistArt saves a manually-uploaded photo (TDR 007), bypassing
// MusicBrainz/Cover Art Archive entirely — synchronous, no queueing/polling:
// the response already reflects the new photo.
func handleUploadArtistArt(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid artist id", http.StatusBadRequest)
			return
		}
		data, err := readUploadedImage(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		artist, err := svc.UploadArtistArt(r.Context(), id, data)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, artist)
	}
}

// handleUploadAlbumArt is handleUploadArtistArt's album counterpart.
func handleUploadAlbumArt(svc *library.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid album id", http.StatusBadRequest)
			return
		}
		data, err := readUploadedImage(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		album, err := svc.UploadAlbumArt(r.Context(), id, data)
		if err != nil {
			http.Error(w, err.Error(), libraryErrorStatus(err))
			return
		}
		writeJSON(w, http.StatusOK, album)
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
