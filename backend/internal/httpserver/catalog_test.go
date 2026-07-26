package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// testStoreAndService mirrors testService but also hands back the
// underlying *library.Store, so catalog tests can seed tracks directly
// (InsertTrack isn't exposed through Service — it's part of the scan
// write path) before exercising the HTTP layer.
func testStoreAndService(t *testing.T, roots library.Roots) (*library.Store, *library.Service) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}

	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)

	schema := "test_" + randomSuffix()
	if _, err := conn.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { conn.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if _, err := conn.Exec("SET search_path TO " + schema); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	store := library.NewStore(conn)
	return store, library.NewService(roots, store, noopScanner{})
}

func TestLibraryArtistsListsAndPaginates(t *testing.T) {
	root := t.TempDir()
	store, svc := testStoreAndService(t, library.Roots{root})

	dir, err := svc.AddDirectory(context.Background(), root)
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}
	if err := store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/a", Title: "A", Artist: "Radiohead", Album: "In Rainbows"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

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
	root := t.TempDir()
	store, svc := testStoreAndService(t, library.Roots{root})

	dir, _ := svc.AddDirectory(context.Background(), root)
	if err := store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/a", Title: "A", Artist: "Radiohead", Album: "In Rainbows", Year: 2007}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

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
	root := t.TempDir()
	store, svc := testStoreAndService(t, library.Roots{root})

	dir, _ := svc.AddDirectory(context.Background(), root)
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/a", Title: "A", Artist: "Fleetwood Mac", Album: "Rumours", Year: 1977})
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/b", Title: "B", Artist: "Tycho", Album: "Weather", Year: 2019})

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
	root := t.TempDir()
	store, svc := testStoreAndService(t, library.Roots{root})

	dir, _ := svc.AddDirectory(context.Background(), root)
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/a", Title: "15 Step", Artist: "Radiohead", Album: "In Rainbows", TrackNumber: 1})
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/b", Title: "Bodysnatchers", Artist: "Radiohead", Album: "In Rainbows", TrackNumber: 2})

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
	root := t.TempDir()
	store, svc := testStoreAndService(t, library.Roots{root})

	dir, _ := svc.AddDirectory(context.Background(), root)
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/a", Title: "Sinnerman", Artist: "Nina Simone", Album: "Pastel Blues"})
	store.InsertTrack(context.Background(), scan.Track{DirectoryID: dir.ID, Path: "/b", Title: "Dreams", Artist: "Fleetwood Mac", Album: "Rumours"})

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
