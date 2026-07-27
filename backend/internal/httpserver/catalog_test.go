package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// mustInsertTrackWithFile is mustInsertTrack, but points the track at a
// real file on disk (containing data) rather than a fake /music/... path
// — needed for the streaming endpoint (TDR 015), which actually opens
// and serves the file's bytes.
func mustInsertTrackWithFile(t *testing.T, store *library.Store, importID int64, artist, album, title string, trackNumber, year int, ext string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), title+ext)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("writing test audio file: %v", err)
	}
	if err := store.InsertTrack(context.Background(), organize.CopiedTrack{
		ImportID: importID, Path: path,
		Title: title, Artist: artist, Album: album, TrackNumber: trackNumber, Year: year,
	}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	return path
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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

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
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if _, err := svc.GetAlbum(context.Background(), id); err == nil {
		t.Fatal("expected album to be removed")
	}
}

func TestSetArtistPrimaryPhotoEndpointSwitchesPrimary(t *testing.T) {
	store, svc := testStoreAndService(t)
	svc.SetImages(enrich.NewImageStore(t.TempDir()))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Gallery Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	if _, err := svc.UploadArtistArt(context.Background(), id, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	second, err := store.AddArtistPhoto(context.Background(), id, "/artwork/artist/x/thumb.jpg", "/artwork/artist/x/full.jpg", "upload", "hash-b")
	if err != nil {
		t.Fatalf("AddArtistPhoto: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/photos/"+strconv.FormatInt(second.ID, 10)+"/primary", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.ArtistDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.PhotoThumbURL != "/artwork/artist/x/thumb.jpg" {
		t.Fatalf("PhotoThumbURL = %q, want the newly-primary photo's thumb", got.PhotoThumbURL)
	}
	if len(got.Photos) != 2 {
		t.Fatalf("len(Photos) = %d, want 2", len(got.Photos))
	}
}

func TestSetArtistPrimaryPhotoEndpointNotFound(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Gallery Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	req := httptest.NewRequest(http.MethodPost, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/photos/999999/primary", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDeleteArtistPhotoEndpointRequiresDeleteFileParam(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/1/photos/1", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestDeleteArtistPhotoEndpointRemovesFileWhenRequested(t *testing.T) {
	store, svc := testStoreAndService(t)
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Gallery Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	if _, err := svc.UploadArtistArt(context.Background(), id, onePixelPNG()); err != nil {
		t.Fatalf("UploadArtistArt: %v", err)
	}
	photos, err := store.ListArtistPhotos(context.Background(), id)
	if err != nil || len(photos) != 1 {
		t.Fatalf("ListArtistPhotos: photos=%+v err=%v", photos, err)
	}
	diskPath := dir + photos[0].ThumbURL[len("/artwork"):]

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/photos/"+strconv.FormatInt(photos[0].ID, 10)+"?deleteFile=true", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.ArtistDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got.Photos) != 0 {
		t.Fatalf("Photos = %+v, want empty after delete", got.Photos)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat error: %v", diskPath, err)
	}
}

func TestDeleteArtistPhotoEndpointNotFound(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Gallery Artist", "Album", "Song", 1, 2020)
	artists, _ := svc.ListArtists(context.Background(), library.ListOptions{})
	id := artists.Items[0].ID

	req := httptest.NewRequest(http.MethodDelete, "/api/library/artists/"+strconv.FormatInt(id, 10)+"/photos/999999?deleteFile=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestSetAlbumPrimaryCoverEndpointSwitchesPrimary(t *testing.T) {
	store, svc := testStoreAndService(t)
	svc.SetImages(enrich.NewImageStore(t.TempDir()))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Artist", "Gallery Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	if _, err := svc.UploadAlbumArt(context.Background(), id, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	second, err := store.AddAlbumCover(context.Background(), id, "/artwork/album/x/thumb.jpg", "/artwork/album/x/full.jpg", "upload", "", "hash-b")
	if err != nil {
		t.Fatalf("AddAlbumCover: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/library/albums/"+strconv.FormatInt(id, 10)+"/covers/"+strconv.FormatInt(second.ID, 10)+"/primary", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.AlbumDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if got.CoverThumbURL != "/artwork/album/x/thumb.jpg" {
		t.Fatalf("CoverThumbURL = %q, want the newly-primary cover's thumb", got.CoverThumbURL)
	}
	if len(got.Covers) != 2 {
		t.Fatalf("len(Covers) = %d, want 2", len(got.Covers))
	}
}

func TestDeleteAlbumCoverEndpointRemovesFileWhenRequested(t *testing.T) {
	store, svc := testStoreAndService(t)
	dir := t.TempDir()
	svc.SetImages(enrich.NewImageStore(dir))
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Artist", "Gallery Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	if _, err := svc.UploadAlbumArt(context.Background(), id, onePixelPNG()); err != nil {
		t.Fatalf("UploadAlbumArt: %v", err)
	}
	covers, err := store.ListAlbumCovers(context.Background(), id)
	if err != nil || len(covers) != 1 {
		t.Fatalf("ListAlbumCovers: covers=%+v err=%v", covers, err)
	}
	diskPath := dir + covers[0].ThumbURL[len("/artwork"):]

	req := httptest.NewRequest(http.MethodDelete, "/api/library/albums/"+strconv.FormatInt(id, 10)+"/covers/"+strconv.FormatInt(covers[0].ID, 10)+"?deleteFile=true", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got library.AlbumDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, rec.Body.String())
	}
	if len(got.Covers) != 0 {
		t.Fatalf("Covers = %+v, want empty after delete", got.Covers)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("expected file %q to be removed, stat error: %v", diskPath, err)
	}
}

func TestDeleteAlbumCoverEndpointNotFound(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Artist", "Gallery Album", "Song", 1, 2020)
	albums, _ := svc.ListAlbums(context.Background(), library.ListOptions{})
	id := albums.Items[0].ID

	req := httptest.NewRequest(http.MethodDelete, "/api/library/albums/"+strconv.FormatInt(id, 10)+"/covers/999999?deleteFile=false", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestStreamSongEndpointServesFullFile(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	data := bytes.Repeat([]byte("abcdefgh"), 100) // 800 bytes
	mustInsertTrackWithFile(t, store, importID, "Stream Artist", "Stream Album", "Stream Song", 1, 2020, ".mp3", data)

	page, err := svc.ListSongs(context.Background(), library.ListOptions{Query: "Stream Song"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListSongs: items=%+v err=%v", page.Items, err)
	}
	id := page.Items[0].ID

	req := httptest.NewRequest(http.MethodGet, "/api/library/songs/"+strconv.FormatInt(id, 10)+"/stream", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Fatalf("Content-Type = %q, want audio/mpeg", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("body length = %d, want %d matching the source file", rec.Body.Len(), len(data))
	}
	if ar := rec.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", ar)
	}
}

func TestStreamSongEndpointSupportsRangeRequests(t *testing.T) {
	store, svc := testStoreAndService(t)
	importID := mustCreateImportForTest(t, store)
	data := bytes.Repeat([]byte("0123456789"), 50) // 500 bytes
	mustInsertTrackWithFile(t, store, importID, "Range Artist", "Range Album", "Range Song", 1, 2020, ".flac", data)

	page, err := svc.ListSongs(context.Background(), library.ListOptions{Query: "Range Song"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListSongs: items=%+v err=%v", page.Items, err)
	}
	id := page.Items[0].ID

	req := httptest.NewRequest(http.MethodGet, "/api/library/songs/"+strconv.FormatInt(id, 10)+"/stream", nil)
	req.Header.Set("Range", "bytes=100-199")
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusPartialContent, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/flac" {
		t.Fatalf("Content-Type = %q, want audio/flac", ct)
	}
	wantRange := "bytes 100-199/500"
	if cr := rec.Header().Get("Content-Range"); cr != wantRange {
		t.Fatalf("Content-Range = %q, want %q", cr, wantRange)
	}
	if !bytes.Equal(rec.Body.Bytes(), data[100:200]) {
		t.Fatalf("range body = %q, want %q", rec.Body.Bytes(), data[100:200])
	}
}

func TestStreamSongEndpointNotFound(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/songs/999999/stream", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestStreamSongEndpointRejectsInvalidID(t *testing.T) {
	_, svc := testStoreAndService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/songs/not-a-number/stream", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
