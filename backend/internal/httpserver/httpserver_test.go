package httpserver

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/lib/pq"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// lazyService builds a *library.Service for tests that never actually hit
// the database (health, static file serving, roots, browse) — its store
// wraps a lazily-connecting *sql.DB that's never queried.
func lazyService(t *testing.T, roots library.Roots) *library.Service {
	t.Helper()
	sqlDB, err := sql.Open("postgres", "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return library.NewService(roots, t.TempDir(), library.NewStore(sqlDB), organize.CopyJob{})
}

func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// testService builds a *library.Service backed by a real, freshly-migrated
// Postgres schema, skipping the test if DATABASE_URL isn't configured.
func testService(t *testing.T, roots library.Roots) *library.Service {
	t.Helper()
	_, svc := testStoreAndService(t, roots)
	return svc
}

// testStoreAndService is testService but also hands back the underlying
// *library.Store, for tests that need to seed tracks directly.
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
	return store, library.NewService(roots, t.TempDir(), store, organize.CopyJob{})
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if body := rec.Body.String(); body != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestHealthReportsRevision(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	New("", "", "abc1234", lazyService(t, nil)).ServeHTTP(rec, req)

	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Revision != "abc1234" {
		t.Fatalf("revision = %q, want %q", body.Revision, "abc1234")
	}
}

func TestStaticFallsBackToIndexForUnknownRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>root</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	rec := httptest.NewRecorder()

	New(dir, "", "", lazyService(t, nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "<html>root</html>" {
		t.Fatalf("body = %q", body)
	}
}

func TestArtworkServesFilesFromArtworkDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "album", "42"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "album", "42", "thumb.jpg"), []byte("fake-jpeg-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/artwork/album/42/thumb.jpg", nil)
	rec := httptest.NewRecorder()

	New("", dir, "", lazyService(t, nil)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "fake-jpeg-bytes" {
		t.Fatalf("body = %q", body)
	}
}

func TestArtworkRouteAbsentWhenArtworkDirUnset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/artwork/album/42/thumb.jpg", nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, nil)).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected the /artwork/ route to be absent when artworkDir is unset, got status %d", rec.Code)
	}
}

func TestImportRootsListsConfiguredRoots(t *testing.T) {
	roots := library.Roots{"/music", "/nas-music"}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/roots", nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var got []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	if len(got) != 2 || got[0].Path != "/music" || got[1].Path != "/nas-music" {
		t.Fatalf("roots = %+v, want [/music, /nas-music]", got)
	}
}

func TestImportBrowseListsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+root, nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got []library.Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	if len(got) != 1 || got[0].Name != "Rock" {
		t.Fatalf("entries = %+v, want [Rock]", got)
	}
}

func TestImportBrowseRejectsPathOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+outside, nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestImportBrowseRequiresPathParam(t *testing.T) {
	roots := library.Roots{t.TempDir()}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse", nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestImportBrowseNonexistentPath(t *testing.T) {
	root := t.TempDir()
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+filepath.Join(root, "nope"), nil)
	rec := httptest.NewRecorder()

	New("", "", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
