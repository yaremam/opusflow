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

func TestWithinRootAllowsAnyPathWhenRootIsEmpty(t *testing.T) {
	if err := WithinRoot("/anything/at/all", ""); err != nil {
		t.Fatalf("WithinRoot with empty root = %v, want nil (unrestricted)", err)
	}
}

func TestWithinRootAcceptsRootItself(t *testing.T) {
	if err := WithinRoot("/data", "/data"); err != nil {
		t.Fatalf("WithinRoot(root, root) = %v, want nil", err)
	}
}

func TestWithinRootAcceptsNestedPath(t *testing.T) {
	if err := WithinRoot("/data/music/Rock", "/data"); err != nil {
		t.Fatalf("WithinRoot(nested, root) = %v, want nil", err)
	}
}

func TestWithinRootRejectsPathOutsideRoot(t *testing.T) {
	err := WithinRoot("/etc/passwd", "/data")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("WithinRoot(outside, root) = %v, want ErrOutsideRoot", err)
	}
}

func TestCreateFolderMakesNewSubdirectory(t *testing.T) {
	parent := t.TempDir()

	entry, err := CreateFolder(parent, "New Library")
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	want := filepath.Join(parent, "New Library")
	if entry.Path != want || entry.Name != "New Library" {
		t.Fatalf("entry = %+v, want {Name: New Library, Path: %s}", entry, want)
	}
	if info, err := os.Stat(want); err != nil || !info.IsDir() {
		t.Fatalf("CreateFolder did not create a directory at %s (err=%v)", want, err)
	}
}

func TestCreateFolderIsIdempotentForAnExistingDirectory(t *testing.T) {
	parent := t.TempDir()
	existing := filepath.Join(parent, "Already Here")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := CreateFolder(parent, "Already Here"); err != nil {
		t.Fatalf("CreateFolder on existing dir: %v", err)
	}
}

func TestCreateFolderRejectsExistingFile(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "afile"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := CreateFolder(parent, "afile"); !errors.Is(err, ErrNotADirectory) {
		t.Fatalf("CreateFolder over an existing file: err = %v, want ErrNotADirectory", err)
	}
}

func TestCreateFolderRejectsNonexistentParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "does-not-exist")

	if _, err := CreateFolder(parent, "New Library"); err == nil {
		t.Fatal("CreateFolder with nonexistent parent: error = nil, want error")
	}
}

func TestCreateFolderRejectsNameWithPathSeparator(t *testing.T) {
	parent := t.TempDir()

	if _, err := CreateFolder(parent, "a/b"); !errors.Is(err, ErrInvalidFolderName) {
		t.Fatalf("CreateFolder(%q): err = %v, want ErrInvalidFolderName", "a/b", err)
	}
}

func TestCreateFolderRejectsParentReference(t *testing.T) {
	parent := t.TempDir()

	if _, err := CreateFolder(parent, ".."); !errors.Is(err, ErrInvalidFolderName) {
		t.Fatalf("CreateFolder(..): err = %v, want ErrInvalidFolderName", err)
	}
}

func TestWithinRootRejectsSiblingWithSharedStringPrefix(t *testing.T) {
	// /data-other must not be treated as inside /data just because it
	// shares a string prefix — this must compare path segments, not bytes.
	err := WithinRoot("/data-other/thing", "/data")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("WithinRoot(/data-other/thing, /data) = %v, want ErrOutsideRoot", err)
	}
}
