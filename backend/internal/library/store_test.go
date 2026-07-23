package library

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library/scan"
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

func TestAddDirectory(t *testing.T) {
	s := testStore(t)

	dir, err := s.AddDirectory(ctx(), "/music", "/music/Rock")
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}
	if dir.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if dir.Root != "/music" || dir.Path != "/music/Rock" {
		t.Fatalf("dir = %+v", dir)
	}
	if dir.Status != StatusScanning {
		t.Fatalf("Status = %q, want %q", dir.Status, StatusScanning)
	}
	if dir.FilesProcessed != 0 || dir.FilesTotal != 0 {
		t.Fatalf("expected zeroed counters, got %+v", dir)
	}
}

func TestAddDirectoryRejectsExactDuplicate(t *testing.T) {
	s := testStore(t)

	if _, err := s.AddDirectory(ctx(), "/music", "/music/Rock"); err != nil {
		t.Fatalf("first AddDirectory: %v", err)
	}
	_, err := s.AddDirectory(ctx(), "/music", "/music/Rock")
	if !errors.Is(err, ErrDuplicateDirectory) {
		t.Fatalf("second AddDirectory error = %v, want ErrDuplicateDirectory", err)
	}
}

func TestAddDirectoryAllowsOverlappingPaths(t *testing.T) {
	s := testStore(t)

	if _, err := s.AddDirectory(ctx(), "/music", "/music/Rock"); err != nil {
		t.Fatalf("AddDirectory(Rock): %v", err)
	}
	// A parent of an existing entry is allowed per AC-2.
	if _, err := s.AddDirectory(ctx(), "/music", "/music"); err != nil {
		t.Fatalf("AddDirectory(parent): %v", err)
	}
}

func TestGetDirectoryNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetDirectory(ctx(), 999999)
	if !errors.Is(err, ErrDirectoryNotFound) {
		t.Fatalf("GetDirectory error = %v, want ErrDirectoryNotFound", err)
	}
}

func TestUpdateProgress(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	if err := s.UpdateProgress(ctx(), dir.ID, 42, 100); err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}

	got, err := s.GetDirectory(ctx(), dir.ID)
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if got.FilesProcessed != 42 || got.FilesTotal != 100 {
		t.Fatalf("got = %+v, want FilesProcessed=42 FilesTotal=100", got)
	}
	if got.Status != StatusScanning {
		t.Fatalf("Status = %q, want %q (progress update shouldn't change status)", got.Status, StatusScanning)
	}
}

func TestMarkComplete(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	if err := s.MarkComplete(ctx(), dir.ID); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}

	got, err := s.GetDirectory(ctx(), dir.ID)
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if got.Status != StatusComplete {
		t.Fatalf("Status = %q, want %q", got.Status, StatusComplete)
	}
}

func TestMarkFailed(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	if err := s.MarkFailed(ctx(), dir.ID, "directory became unreadable"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	got, err := s.GetDirectory(ctx(), dir.ID)
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusFailed)
	}
	if got.Error != "directory became unreadable" {
		t.Fatalf("Error = %q", got.Error)
	}
}

func TestInsertTrackAndTrackCount(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	track := scan.Track{
		DirectoryID:     dir.ID,
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

	got, err := s.GetDirectory(ctx(), dir.ID)
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if got.TrackCount != 1 {
		t.Fatalf("TrackCount = %d, want 1", got.TrackCount)
	}

	dirs, err := s.ListDirectories(ctx())
	if err != nil {
		t.Fatalf("ListDirectories: %v", err)
	}
	if len(dirs) != 1 || dirs[0].TrackCount != 1 {
		t.Fatalf("ListDirectories = %+v, want one dir with TrackCount=1", dirs)
	}
}

func TestRecordFileErrorSurfacesOnDirectory(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	if err := s.RecordFileError(ctx(), dir.ID, "/music/Rock/bad.mp3", "corrupt tag"); err != nil {
		t.Fatalf("RecordFileError: %v", err)
	}

	got, err := s.GetDirectory(ctx(), dir.ID)
	if err != nil {
		t.Fatalf("GetDirectory: %v", err)
	}
	if len(got.FileErrors) != 1 {
		t.Fatalf("FileErrors = %+v, want 1 entry", got.FileErrors)
	}
	if got.FileErrors[0].Path != "/music/Rock/bad.mp3" || got.FileErrors[0].Error != "corrupt tag" {
		t.Fatalf("FileErrors[0] = %+v", got.FileErrors[0])
	}
}

func TestRemoveDirectoryCascadesTracksAndErrors(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")
	if err := s.InsertTrack(ctx(), scan.Track{DirectoryID: dir.ID, Path: "/music/Rock/song.mp3", Title: "Song"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := s.RecordFileError(ctx(), dir.ID, "/music/Rock/bad.mp3", "corrupt"); err != nil {
		t.Fatalf("RecordFileError: %v", err)
	}

	if err := s.RemoveDirectory(ctx(), dir.ID); err != nil {
		t.Fatalf("RemoveDirectory: %v", err)
	}

	if _, err := s.GetDirectory(ctx(), dir.ID); !errors.Is(err, ErrDirectoryNotFound) {
		t.Fatalf("GetDirectory after remove = %v, want ErrDirectoryNotFound", err)
	}

	var trackCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tracks WHERE directory_id = $1`, dir.ID).Scan(&trackCount); err != nil {
		t.Fatalf("counting tracks: %v", err)
	}
	if trackCount != 0 {
		t.Fatalf("tracks remaining after cascade delete = %d, want 0", trackCount)
	}
}

func TestRemoveDirectoryNotFound(t *testing.T) {
	s := testStore(t)

	err := s.RemoveDirectory(ctx(), 999999)
	if !errors.Is(err, ErrDirectoryNotFound) {
		t.Fatalf("RemoveDirectory error = %v, want ErrDirectoryNotFound", err)
	}
}
