package library

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// testStore opens a per-test schema against a real Postgres instance,
// migrated fresh, skipping if DATABASE_URL isn't configured.
func testStore(t *testing.T) *Store {
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

	// Force every query in this test onto a single underlying connection,
	// since SET search_path is connection-scoped and the pool would
	// otherwise silently hand out a second, unscoped connection.
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

	return NewStore(conn)
}

func ctx() context.Context { return context.Background() }

// mustCreateImport creates an import with a throwaway source description,
// for tests that just need a valid import_id to attribute tracks to.
func mustCreateImport(t *testing.T, s *Store) int64 {
	t.Helper()
	imp, err := s.CreateImport(ctx(), "/music/"+randomSuffix())
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	return imp.ID
}

func TestCreateImport(t *testing.T) {
	s := testStore(t)

	imp, err := s.CreateImport(ctx(), "/music/Rock")
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	if imp.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if imp.SourceDescription != "/music/Rock" {
		t.Fatalf("SourceDescription = %q", imp.SourceDescription)
	}
	if imp.Status != ImportStatusCopying {
		t.Fatalf("Status = %q, want %q", imp.Status, ImportStatusCopying)
	}
	if imp.FilesProcessed != 0 || imp.FilesTotal != 0 {
		t.Fatalf("expected zeroed counters, got %+v", imp)
	}
}

func TestGetImportNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetImport(ctx(), 999999)
	if !errors.Is(err, ErrImportNotFound) {
		t.Fatalf("GetImport error = %v, want ErrImportNotFound", err)
	}
}

func TestUpdateImportProgress(t *testing.T) {
	s := testStore(t)
	imp, _ := s.CreateImport(ctx(), "/music/Rock")

	if err := s.UpdateImportProgress(ctx(), imp.ID, 42, 100); err != nil {
		t.Fatalf("UpdateImportProgress: %v", err)
	}

	got, err := s.GetImport(ctx(), imp.ID)
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got.FilesProcessed != 42 || got.FilesTotal != 100 {
		t.Fatalf("got = %+v, want FilesProcessed=42 FilesTotal=100", got)
	}
	if got.Status != ImportStatusCopying {
		t.Fatalf("Status = %q, want %q (progress update shouldn't change status)", got.Status, ImportStatusCopying)
	}
}

func TestMarkImportComplete(t *testing.T) {
	s := testStore(t)
	imp, _ := s.CreateImport(ctx(), "/music/Rock")

	if err := s.MarkImportComplete(ctx(), imp.ID); err != nil {
		t.Fatalf("MarkImportComplete: %v", err)
	}

	got, err := s.GetImport(ctx(), imp.ID)
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got.Status != ImportStatusComplete {
		t.Fatalf("Status = %q, want %q", got.Status, ImportStatusComplete)
	}
}

func TestMarkImportFailed(t *testing.T) {
	s := testStore(t)
	imp, _ := s.CreateImport(ctx(), "/music/Rock")

	if err := s.MarkImportFailed(ctx(), imp.ID, "every file failed to copy"); err != nil {
		t.Fatalf("MarkImportFailed: %v", err)
	}

	got, err := s.GetImport(ctx(), imp.ID)
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got.Status != ImportStatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, ImportStatusFailed)
	}
	if got.Error != "every file failed to copy" {
		t.Fatalf("Error = %q", got.Error)
	}
}

func TestInsertTrackAndTrackCount(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)

	track := organize.CopiedTrack{
		ImportID:        importID,
		Path:            "/music/Rock/song.mp3",
		Title:           "Song",
		Artist:          "Artist",
		Album:           "Album",
		TrackNumber:     3,
		Year:            1999,
		Genre:           "Rock",
		DurationSeconds: 215,
	}
	if err := s.InsertTrack(ctx(), track); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	got, err := s.GetImport(ctx(), importID)
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if got.TrackCount != 1 {
		t.Fatalf("TrackCount = %d, want 1", got.TrackCount)
	}
}

func TestListImportsNewestFirst(t *testing.T) {
	s := testStore(t)
	first, _ := s.CreateImport(ctx(), "/music/first")
	second, _ := s.CreateImport(ctx(), "/music/second")

	imports, err := s.ListImports(ctx())
	if err != nil {
		t.Fatalf("ListImports: %v", err)
	}
	if len(imports) != 2 || imports[0].ID != second.ID || imports[1].ID != first.ID {
		t.Fatalf("imports = %+v, want [second, first]", imports)
	}
}

func TestRecordImportErrorSurfacesOnImport(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)

	if err := s.RecordImportError(ctx(), importID, "/music/Rock/bad.mp3", "corrupt tag"); err != nil {
		t.Fatalf("RecordImportError: %v", err)
	}

	got, err := s.GetImport(ctx(), importID)
	if err != nil {
		t.Fatalf("GetImport: %v", err)
	}
	if len(got.FileErrors) != 1 {
		t.Fatalf("FileErrors = %+v, want 1 entry", got.FileErrors)
	}
	if got.FileErrors[0].Path != "/music/Rock/bad.mp3" || got.FileErrors[0].Error != "corrupt tag" {
		t.Fatalf("FileErrors[0] = %+v", got.FileErrors[0])
	}
}
