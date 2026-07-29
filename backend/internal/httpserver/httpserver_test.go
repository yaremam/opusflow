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
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// lazyService builds a *library.Service for tests that never actually hit
// the database (health, static file serving, browse) — its store wraps a
// lazily-connecting *sql.DB that's never queried.
func lazyService(t *testing.T) *library.Service {
	t.Helper()
	sqlDB, err := sql.Open("postgres", "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return library.NewService(library.NewStore(sqlDB), organize.CopyJob{})
}

func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// testService builds a *library.Service backed by a real, freshly-migrated
// Postgres schema, skipping the test if DATABASE_URL isn't configured.
func testService(t *testing.T) *library.Service {
	t.Helper()
	_, svc := testStoreAndService(t)
	return svc
}

// testStoreAndService is testService but also hands back the underlying
// *library.Store, for tests that need to seed tracks directly.
func testStoreAndService(t *testing.T) (*library.Store, *library.Service) {
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
	return store, library.NewService(store, organize.CopyJob{})
}

func TestHealth(t *testing.T) {
	for _, path := range []string{"/health", "/api/health"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", ct)
			}
			if body := rec.Body.String(); body != "{\"status\":\"ok\"}\n" {
				t.Fatalf("body = %q", body)
			}
		})
	}
}

func TestAboutReportsVersionAndBuildDate(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
	rec := httptest.NewRecorder()

	New("", "", "v0.1.0-4-gabc1234", "2026-07-27T14:32:00Z", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body aboutResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != "v0.1.0-4-gabc1234" {
		t.Fatalf("version = %q, want %q", body.Version, "v0.1.0-4-gabc1234")
	}
	if body.BuildDate != "2026-07-27T14:32:00Z" {
		t.Fatalf("buildDate = %q, want %q", body.BuildDate, "2026-07-27T14:32:00Z")
	}
}

func TestStaticFallsBackToIndexForUnknownRoute(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>root</html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	rec := httptest.NewRecorder()

	New(dir, "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

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

	New("", dir, "", "", lazyService(t), nil).ServeHTTP(rec, req)

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

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected the /artwork/ route to be absent when artworkDir is unset, got status %d", rec.Code)
	}
}

func TestConfigReportsEmptyDataDirWhenUnconfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var got struct {
		DataDir string `json:"dataDir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	if got.DataDir != "" {
		t.Fatalf("dataDir = %q, want empty", got.DataDir)
	}
}

func TestConfigReportsConfiguredDataDir(t *testing.T) {
	svc := lazyService(t)
	svc.SetBrowseRoot("/data")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", svc, nil).ServeHTTP(rec, req)

	var got struct {
		DataDir string `json:"dataDir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	if got.DataDir != "/data" {
		t.Fatalf("dataDir = %q, want /data", got.DataDir)
	}
}

func TestImportBrowseListsSubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Rock"), 0o755); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+root, nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

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

func TestImportBrowseAllowsAnyPath(t *testing.T) {
	// No allowlist anymore (TDR 006) — any absolute path is browsable.
	root := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+root, nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestImportBrowseRejectsPathOutsideConfiguredRoot(t *testing.T) {
	svc := lazyService(t)
	svc.SetBrowseRoot(t.TempDir())

	outside := t.TempDir() // a sibling temp dir, not under the configured root
	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+outside, nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", svc, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestImportBrowseRequiresPathParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse", nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestImportBrowseNonexistentPath(t *testing.T) {
	root := t.TempDir()

	req := httptest.NewRequest(http.MethodGet, "/api/imports/browse?path="+filepath.Join(root, "nope"), nil)
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestCreateFolderEndpointMakesNewSubdirectory(t *testing.T) {
	parent := t.TempDir()
	body := `{"parentPath":"` + jsonEscape(parent) + `","name":"New Library"}`

	req := httptest.NewRequest(http.MethodPost, "/api/imports/browse/folders", strings.NewReader(body))
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var entry library.Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal response: %v, body = %s", err, rec.Body.String())
	}
	want := filepath.Join(parent, "New Library")
	if entry.Path != want {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("folder not created at %s (err=%v)", want, err)
	}
}

func TestCreateFolderEndpointRejectsPathOutsideConfiguredRoot(t *testing.T) {
	svc := lazyService(t)
	svc.SetBrowseRoot(t.TempDir())

	outside := t.TempDir()
	body := `{"parentPath":"` + jsonEscape(outside) + `","name":"New Library"}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports/browse/folders", strings.NewReader(body))
	rec := httptest.NewRecorder()

	New("", "", "", "", svc, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreateFolderEndpointRequiresName(t *testing.T) {
	parent := t.TempDir()
	body := `{"parentPath":"` + jsonEscape(parent) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/imports/browse/folders", strings.NewReader(body))
	rec := httptest.NewRecorder()

	New("", "", "", "", lazyService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
