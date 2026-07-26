package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// setupTree builds:
//
//	<base>/music/Rock/           (dir)
//	<base>/music/Jazz/           (dir)
//	<base>/music/Rock/song.mp3   (file, should not appear as an entry)
func setupTree(t *testing.T) (musicRoot string) {
	t.Helper()
	base := t.TempDir()
	musicRoot = filepath.Join(base, "music")

	for _, dir := range []string{
		filepath.Join(musicRoot, "Rock"),
		filepath.Join(musicRoot, "Jazz"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(musicRoot, "Rock", "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return musicRoot
}

func TestBrowseListsSubdirectories(t *testing.T) {
	musicRoot := setupTree(t)

	entries, err := Browse(musicRoot)
	if err != nil {
		t.Fatalf("Browse(%q) error = %v", musicRoot, err)
	}

	if len(entries) != 2 {
		t.Fatalf("Browse(%q) = %v, want 2 entries", musicRoot, entries)
	}
	if entries[0].Name != "Jazz" || entries[1].Name != "Rock" {
		t.Fatalf("Browse(%q) = %v, want [Jazz, Rock] sorted", musicRoot, entries)
	}
	if entries[1].Path != filepath.Join(musicRoot, "Rock") {
		t.Fatalf("entries[1].Path = %q, want %q", entries[1].Path, filepath.Join(musicRoot, "Rock"))
	}
}

func TestBrowseExcludesFiles(t *testing.T) {
	musicRoot := setupTree(t)

	entries, err := Browse(filepath.Join(musicRoot, "Rock"))
	if err != nil {
		t.Fatalf("Browse error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Browse(Rock) = %v, want no entries (song.mp3 is a file, not a dir)", entries)
	}
}

func TestBrowseNestedSubdirectory(t *testing.T) {
	musicRoot := setupTree(t)

	entries, err := Browse(filepath.Join(musicRoot, "Jazz"))
	if err != nil {
		t.Fatalf("Browse error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Browse(Jazz) = %v, want empty (no subdirectories)", entries)
	}
}

func TestBrowseAllowsAnyPath(t *testing.T) {
	// No allowlist anymore (TDR 006) — a sibling directory unrelated to
	// musicRoot is just as browsable.
	base := t.TempDir()
	other := filepath.Join(base, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Browse(other); err != nil {
		t.Fatalf("Browse(%q) error = %v, want nil", other, err)
	}
}

func TestBrowseNonexistentPath(t *testing.T) {
	musicRoot := setupTree(t)

	_, err := Browse(filepath.Join(musicRoot, "does-not-exist"))
	if err == nil {
		t.Fatal("Browse(nonexistent) error = nil, want error")
	}
}

func TestValidateDirectoryAcceptsExistingDirectory(t *testing.T) {
	musicRoot := setupTree(t)

	if err := ValidateDirectory(musicRoot); err != nil {
		t.Fatalf("ValidateDirectory(%q) error = %v, want nil", musicRoot, err)
	}
}

func TestValidateDirectoryRejectsAFile(t *testing.T) {
	musicRoot := setupTree(t)
	file := filepath.Join(musicRoot, "Rock", "song.mp3")

	err := ValidateDirectory(file)
	if !errors.Is(err, ErrNotADirectory) {
		t.Fatalf("ValidateDirectory(%q) error = %v, want ErrNotADirectory", file, err)
	}
}

func TestValidateDirectoryRejectsNonexistentPath(t *testing.T) {
	musicRoot := setupTree(t)
	missing := filepath.Join(musicRoot, "does-not-exist")

	if err := ValidateDirectory(missing); err == nil {
		t.Fatal("ValidateDirectory(nonexistent) error = nil, want error")
	}
}
