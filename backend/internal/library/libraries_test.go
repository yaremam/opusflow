package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

func mustCreateLibrary(t *testing.T, s *Store, rootPath string) Library {
	t.Helper()
	lib, err := s.CreateLibrary(context.Background(), "Test Library "+randomSuffix(), rootPath)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	return lib
}

func TestCreateLibrary(t *testing.T) {
	s := testStore(t)
	root := t.TempDir()

	lib, err := s.CreateLibrary(ctx(), "Main Collection", root)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if lib.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if lib.Name != "Main Collection" || lib.RootPath != root {
		t.Fatalf("lib = %+v", lib)
	}
}

func TestCreateLibraryRejectsNonexistentPath(t *testing.T) {
	s := testStore(t)

	_, err := s.CreateLibrary(ctx(), "Main Collection", filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Fatal("CreateLibrary(nonexistent path) error = nil, want error")
	}
}

func TestCreateLibraryRejectsAFilePath(t *testing.T) {
	s := testStore(t)
	file := filepath.Join(t.TempDir(), "not-a-dir.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.CreateLibrary(ctx(), "Main Collection", file)
	if !errors.Is(err, ErrNotADirectory) {
		t.Fatalf("CreateLibrary(file path) error = %v, want ErrNotADirectory", err)
	}
}

func TestListLibrariesIncludesTrackCount(t *testing.T) {
	s := testStore(t)
	lib := mustCreateLibrary(t, s, t.TempDir())
	imp, err := s.CreateImport(ctx(), lib.ID, "/some/source")
	if err != nil {
		t.Fatalf("CreateImport: %v", err)
	}
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: imp.ID, Path: "/x.mp3", Title: "T", Artist: "A", Album: "B"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	libs, err := s.ListLibraries(ctx())
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	var found *Library
	for i := range libs {
		if libs[i].ID == lib.ID {
			found = &libs[i]
		}
	}
	if found == nil {
		t.Fatalf("library %d not found in %+v", lib.ID, libs)
	}
	if found.TrackCount != 1 {
		t.Fatalf("TrackCount = %d, want 1", found.TrackCount)
	}
}

func TestGetLibraryNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetLibrary(ctx(), 999999)
	if !errors.Is(err, ErrLibraryNotFound) {
		t.Fatalf("GetLibrary error = %v, want ErrLibraryNotFound", err)
	}
}

func TestDeleteLibraryNotFound(t *testing.T) {
	s := testStore(t)

	err := s.DeleteLibrary(ctx(), 999999, false)
	if !errors.Is(err, ErrLibraryNotFound) {
		t.Fatalf("DeleteLibrary error = %v, want ErrLibraryNotFound", err)
	}
}

func TestDeleteLibraryRemovesTracksAndOrphanedCatalogEntries(t *testing.T) {
	s := testStore(t)
	lib := mustCreateLibrary(t, s, t.TempDir())
	imp, _ := s.CreateImport(ctx(), lib.ID, "/some/source")
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: imp.ID, Path: "/x.mp3", Title: "Song", Artist: "Solo Artist", Album: "Solo Album"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := s.DeleteLibrary(ctx(), lib.ID, false); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	if _, err := s.GetLibrary(ctx(), lib.ID); !errors.Is(err, ErrLibraryNotFound) {
		t.Fatalf("GetLibrary after delete = %v, want ErrLibraryNotFound", err)
	}
	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Solo Artist"})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 0 {
		t.Fatalf("artists after library delete = %+v, want none (orphan should be deleted)", artists.Items)
	}
}

func TestDeleteLibraryKeepsArtistStillReferencedByAnotherLibrary(t *testing.T) {
	s := testStore(t)
	libA := mustCreateLibrary(t, s, t.TempDir())
	libB := mustCreateLibrary(t, s, t.TempDir())
	impA, _ := s.CreateImport(ctx(), libA.ID, "/source-a")
	impB, _ := s.CreateImport(ctx(), libB.ID, "/source-b")

	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: impA.ID, Path: "/a.mp3", Title: "Song A", Artist: "Shared Artist", Album: "Album One"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: impB.ID, Path: "/b.mp3", Title: "Song B", Artist: "Shared Artist", Album: "Album Two"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := s.DeleteLibrary(ctx(), libA.ID, false); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Shared Artist"})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 || artists.Items[0].TrackCount != 1 {
		t.Fatalf("artists after partial library delete = %+v, want 1 artist with 1 remaining track", artists.Items)
	}
}

func TestDeleteLibraryDeletesFilesWhenRequested(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := mustCreateLibrary(t, s, dir)
	imp, _ := s.CreateImport(ctx(), lib.ID, dir)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: imp.ID, Path: file, Title: "Song", Artist: "A", Album: "B"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := s.DeleteLibrary(ctx(), lib.ID, true); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be removed from disk, stat error: %v", err)
	}
}

func TestDeleteLibraryKeepsFilesWhenNotRequested(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := mustCreateLibrary(t, s, dir)
	imp, _ := s.CreateImport(ctx(), lib.ID, dir)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: imp.ID, Path: file, Title: "Song", Artist: "A", Album: "B"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	if err := s.DeleteLibrary(ctx(), lib.ID, false); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected file to remain on disk, stat error: %v", err)
	}
}
