package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
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
	libID := mustCreateLibraryForTest(t, store, t.TempDir())
	imp, err := store.CreateImport(context.Background(), libID, "/music/src")
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	return imp.ID
}

func TestLibraryArtistsListsAndPaginates(t *testing.T) {
	store, svc := testStoreAndService(t)
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
	store, svc := testStoreAndService(t)
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
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibraryAlbumsFiltersByYear(t *testing.T) {
	store, svc := testStoreAndService(t)
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
	store, svc := testStoreAndService(t)
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
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/albums/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibrarySongsSearchesByQuery(t *testing.T) {
	store, svc := testStoreAndService(t)
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
	store, svc := testStoreAndService(t)
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
	store, svc := testStoreAndService(t)
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

func TestRetryArtistArtEndpointResetsStatusToPending(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Retry Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	if err := store.SetArtistArt(context.Background(), id, "found", "/artwork/artist/1/thumb.jpg", "/artwork/artist/1/full.jpg"); err != nil {
		t.Fatalf("SetArtistArt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/art/retry", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var got library.ArtistDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.ArtStatus != "pending" {
		t.Fatalf("ArtStatus = %q, want pending", got.ArtStatus)
	}
	// The photo URL must survive the reset (AC-11) — only a later Found
	// write may clear it.
	if got.PhotoThumbURL == "" {
		t.Fatal("expected photo URL to survive the retry reset")
	}
}

func TestRetryArtistArtEndpointNotFound(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/999999/art/retry", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestRetryAlbumArtEndpointResetsStatusToPending(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Artist", "Retry Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	if err := store.SetAlbumArt(context.Background(), id, "found", "/artwork/album/1/thumb.jpg", "/artwork/album/1/full.jpg"); err != nil {
		t.Fatalf("SetAlbumArt: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/albums/"+strconv.FormatInt(id, 10)+"/art/retry", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var got library.AlbumDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.ArtStatus != "pending" {
		t.Fatalf("ArtStatus = %q, want pending", got.ArtStatus)
	}
	if got.CoverThumbURL == "" {
		t.Fatal("expected cover URL to survive the retry reset")
	}
}

func TestRetryAlbumArtEndpointNotFound(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodPost, "/api/library/albums/999999/art/retry", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

// onePixelPNG is a minimal valid PNG, for tests that just need something
// image.Decode accepts without caring what it looks like.
func onePixelPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d, 0xb0, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

// mustBuildImageUploadBody builds a multipart request body with a single
// "image" field carrying data, plus the multipart Content-Type header a
// caller needs to set alongside it.
func mustBuildImageUploadBody(t *testing.T, data []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("image", "art.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return body, mw.FormDataContentType()
}

func TestUploadArtistArtEndpointSavesImage(t *testing.T) {
	store, svc := testStoreAndService(t)
	svc.SetImages(enrich.NewImageStore(t.TempDir()))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Upload Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	body, contentType := mustBuildImageUploadBody(t, onePixelPNG())
	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/art", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.Artist
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.ArtStatus != "found" || got.PhotoThumbURL == "" || got.PhotoURL == "" {
		t.Fatalf("artist = %+v, want ArtStatus found with non-empty photo URLs", got)
	}
}

func TestUploadArtistArtEndpointWithoutArtworkConfiguredReturns503(t *testing.T) {
	store, svc := testStoreAndService(t) // no SetImages call
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "No Artwork Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	body, contentType := mustBuildImageUploadBody(t, onePixelPNG())
	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/art", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}
}

func TestUploadArtistArtEndpointRejectsMissingImageField(t *testing.T) {
	_, svc := testStoreAndService(t)
	svc.SetImages(enrich.NewImageStore(t.TempDir()))

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/1/art", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUploadAlbumArtEndpointSavesImage(t *testing.T) {
	store, svc := testStoreAndService(t)
	svc.SetImages(enrich.NewImageStore(t.TempDir()))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Artist", "Upload Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	body, contentType := mustBuildImageUploadBody(t, onePixelPNG())
	req := httptest.NewRequest(http.MethodPost, "/api/library/albums/"+strconv.FormatInt(id, 10)+"/art", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.Album
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.ArtStatus != "found" || got.CoverThumbURL == "" || got.CoverURL == "" {
		t.Fatalf("album = %+v, want ArtStatus found with non-empty cover URLs", got)
	}
}

func TestDeleteArtistEndpointNotFound(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/999999?deleteFiles=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDeleteAlbumEndpointRemovesAlbum(t *testing.T) {
	store, svc := testStoreAndService(t)
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
