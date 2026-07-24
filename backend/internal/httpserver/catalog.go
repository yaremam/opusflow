package httpserver

import (
	"net/http"
	"strconv"

	"github.com/yaremam/opusflow/backend/internal/library"
)

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
