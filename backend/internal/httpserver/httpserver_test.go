package httpserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library"
)

type noopScanner struct{}

func (noopScanner) Scan(context.Context, int64, string) {}

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
	return library.NewService(roots, library.NewStore(sqlDB), noopScanner{})
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

	return library.NewService(roots, library.NewStore(conn), noopScanner{})
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, nil)).ServeHTTP(rec, req)

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

func TestStaticFallsBackToIndexForUnknownRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>root</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	rec := httptest.NewRecorder()

	New(dir, "", lazyService(t, nil)).ServeHTTP(rec, req)

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

	New("", dir, lazyService(t, nil)).ServeHTTP(rec, req)

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

	New("", "", lazyService(t, nil)).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected the /artwork/ route to be absent when artworkDir is unset, got status %d", rec.Code)
	}
}

func TestLibraryRootsListsConfiguredRoots(t *testing.T) {
	roots := library.Roots{"/music", "/nas-music"}

	req := httptest.NewRequest(http.MethodGet, "/api/library/roots", nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, roots)).ServeHTTP(rec, req)

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

func TestLibraryBrowseListsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/library/browse?path="+root, nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, roots)).ServeHTTP(rec, req)

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

func TestLibraryBrowseRejectsPathOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/library/browse?path="+outside, nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestLibraryBrowseRequiresPathParam(t *testing.T) {
	roots := library.Roots{t.TempDir()}

	req := httptest.NewRequest(http.MethodGet, "/api/library/browse", nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestLibraryBrowseNonexistentPath(t *testing.T) {
	root := t.TempDir()
	roots := library.Roots{root}

	req := httptest.NewRequest(http.MethodGet, "/api/library/browse?path="+filepath.Join(root, "nope"), nil)
	rec := httptest.NewRecorder()

	New("", "", lazyService(t, roots)).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibraryDirectoriesListEmpty(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/library/directories", nil)
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestLibraryDirectoriesAdd(t *testing.T) {
	root := t.TempDir()
	svc := testService(t, library.Roots{root})

	reqBody := `{"path":"` + root + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var got library.Directory
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	if got.Path != root || got.Root != root || got.Status != library.StatusScanning {
		t.Fatalf("directory = %+v", got)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/library/directories", nil)
	listRec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(listRec, listReq)

	var list []library.Directory
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list response: %v, body = %s", err, listRec.Body.String())
	}
	if len(list) != 1 || list[0].ID != got.ID {
		t.Fatalf("list = %+v, want one entry with ID %d", list, got.ID)
	}
}

func TestLibraryDirectoriesAddRejectsOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	svc := testService(t, library.Roots{root})

	reqBody := `{"path":"` + outside + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestLibraryDirectoriesAddRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	svc := testService(t, library.Roots{root})

	reqBody := `{"path":"` + root + `"}`
	first := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(reqBody))
	New("", "", svc).ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, second)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
}

func TestLibraryDirectoriesAddRequiresPath(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestLibraryDirectoriesAddRejectsMalformedJSON(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodPost, "/api/library/directories", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestLibraryDirectoriesRemove(t *testing.T) {
	root := t.TempDir()
	svc := testService(t, library.Roots{root})

	dir, err := svc.AddDirectory(context.Background(), root)
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/library/directories/"+strconv.FormatInt(dir.ID, 10), nil)
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	if _, err := svc.GetDirectory(context.Background(), dir.ID); err == nil {
		t.Fatal("expected directory to be removed")
	}
}

func TestLibraryDirectoriesRemoveNotFound(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodDelete, "/api/library/directories/999999", nil)
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestLibraryDirectoriesRemoveInvalidID(t *testing.T) {
	svc := testService(t, library.Roots{t.TempDir()})

	req := httptest.NewRequest(http.MethodDelete, "/api/library/directories/not-a-number", nil)
	rec := httptest.NewRecorder()
	New("", "", svc).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
