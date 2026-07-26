package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

func mustInsertTrack(t *testing.T, store *library.Store, importID int64, artist, album, title string, trackNumber, year int) {
	t.Helper()
	if err := store.InsertTrack(context.Background(), organize.CopiedTrack{
		ImportID: importID, Path: "/music/" + artist + "/" + album + "/" + title + ".mp3",
		Title: title, Artist: artist, Album: album, TrackNumber: trackNumber, Year: year,
	}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
}

func mustCreateImportForTest(t *testing.T, store *library.Store) int64 {
	t.Helper()
	imp, err := store.CreateImport(context.Background(), "/music/src")
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	return imp.ID
}

func TestLibraryArtistsListsAndPaginates(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Radiohead", "In Rainbows", "A", 1, 2007)

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got library.Page[library.Artist]
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.TotalCount != 1 || len(got.Items) != 1 || got.Items[0].Name != "Radiohead" {
		t.Fatalf("page = %+v", got)
	}
	if got.Page != 1 || got.PageSize != 30 {
		t.Fatalf("page/pageSize = %d/%d, want defaults 1/30", got.Page, got.PageSize)
	}
}

func TestLibraryArtistDetailReturnsAlbums(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Radiohead", "In Rainbows", "A", 1, 2007)

	artists, err := svc.ListArtists(context.Background(), library.ListOptions{})
	if err != nil || len(artists.Items) != 1 {
		t.Fatalf("ListArtists: %v, %+v", err, artists)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists/"+strconv.FormatInt(artists.Items[0].ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.ArtistDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "Radiohead" || len(got.Albums) != 1 || got.Albums[0].Title != "In Rainbows" {
		t.Fatalf("detail = %+v", got)
	}
}

func TestLibraryArtistDetailNotFound(t *testing.T) {
	_, svc := testStoreAndService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibraryAlbumsFiltersByYear(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Fleetwood Mac", "Rumours", "A", 1, 1977)
	mustInsertTrack(t, store, importID, "Tycho", "Weather", "B", 1, 2019)

	req := httptest.NewRequest(http.MethodGet, "/api/library/albums?year=1977", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got library.Page[library.Album]
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Title != "Rumours" {
		t.Fatalf("albums = %+v, want just Rumours", got.Items)
	}
}

func TestLibraryAlbumDetailReturnsTracks(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Radiohead", "In Rainbows", "15 Step", 1, 2007)
	mustInsertTrack(t, store, importID, "Radiohead", "In Rainbows", "Bodysnatchers", 2, 2007)

	albums, err := svc.ListAlbums(context.Background(), library.ListOptions{})
	if err != nil || len(albums.Items) != 1 {
		t.Fatalf("ListAlbums: %v, %+v", err, albums)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/library/albums/"+strconv.FormatInt(albums.Items[0].ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got library.AlbumDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Title != "In Rainbows" || len(got.Tracks) != 2 || got.Tracks[0].Title != "15 Step" {
		t.Fatalf("detail = %+v", got)
	}
}

func TestLibraryAlbumDetailNotFound(t *testing.T) {
	_, svc := testStoreAndService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/library/albums/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibrarySongsSearchesByQuery(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Nina Simone", "Pastel Blues", "Sinnerman", 1, 1965)
	mustInsertTrack(t, store, importID, "Fleetwood Mac", "Rumours", "Dreams", 1, 1977)

	req := httptest.NewRequest(http.MethodGet, "/api/library/songs?q=sinner", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got library.Page[library.Song]
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Items) != 1 || got.Items[0].Title != "Sinnerman" {
		t.Fatalf("songs = %+v, want just Sinnerman", got.Items)
	}
}

func TestDeleteArtistEndpointRequiresDeleteFilesParam(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Solo Artist", "Solo Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/"+strconv.FormatInt(artists.Items[0].ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDeleteArtistEndpointRemovesArtist(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Solo Artist", "Solo Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/"+strconv.FormatInt(id, 10)+"?deleteFiles=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if _, err := svc.GetArtist(context.Background(), id); err == nil {
		t.Fatal("expected artist to be removed")
	}
}

func TestDeleteArtistEndpointNotFound(t *testing.T) {
	_, svc := testStoreAndService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/999999?deleteFiles=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDeleteAlbumEndpointRemovesAlbum(t *testing.T) {
	store, svc := testStoreAndService(t, library.Roots{t.TempDir()})
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Solo Artist", "Solo Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	req := httptest.NewRequest(http.MethodDelete, "/api/library/albums/"+strconv.FormatInt(id, 10)+"?deleteFiles=true", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if _, err := svc.GetAlbum(context.Background(), id); err == nil {
		t.Fatal("expected album to be removed")
	}
}
