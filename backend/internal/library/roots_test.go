package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRoots(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want Roots
	}{
		{"single", "/music", Roots{"/music"}},
		{"multiple", "/music,/nas-music", Roots{"/music", "/nas-music"}},
		{"trims whitespace", " /music , /nas-music ", Roots{"/music", "/nas-music"}},
		{"skips empty entries", "/music,,/nas-music,", Roots{"/music", "/nas-music"}},
		{"empty env", "", nil},
		{"cleans trailing slash", "/music/", Roots{"/music"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseRoots(tt.env)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseRoots(%q) = %v, want %v", tt.env, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParseRoots(%q) = %v, want %v", tt.env, got, tt.want)
				}
			}
		})
	}
}

// setupTree builds:
//
//	<base>/music/Rock/           (dir)
//	<base>/music/Jazz/           (dir)
//	<base>/music/Rock/song.mp3   (file, should not appear as an entry)
//	<base>/other/                (dir, sibling root not configured)
func setupTree(t *testing.T) (musicRoot string, otherRoot string) {
	t.Helper()
	base := t.TempDir()

	musicRoot = filepath.Join(base, "music")
	otherRoot = filepath.Join(base, "other")

	for _, dir := range []string{
		filepath.Join(musicRoot, "Rock"),
		filepath.Join(musicRoot, "Jazz"),
		otherRoot,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(musicRoot, "Rock", "song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return musicRoot, otherRoot
}

func TestBrowseListsSubdirectoriesOfRoot(t *testing.T) {
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	entries, err := roots.Browse(musicRoot)
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
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	entries, err := roots.Browse(filepath.Join(musicRoot, "Rock"))
	if err != nil {
		t.Fatalf("Browse error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Browse(Rock) = %v, want no entries (song.mp3 is a file, not a dir)", entries)
	}
}

func TestBrowseNestedSubdirectory(t *testing.T) {
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	entries, err := roots.Browse(filepath.Join(musicRoot, "Jazz"))
	if err != nil {
		t.Fatalf("Browse error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("Browse(Jazz) = %v, want empty (no subdirectories)", entries)
	}
}

func TestBrowseRejectsPathOutsideRoots(t *testing.T) {
	musicRoot, otherRoot := setupTree(t)
	roots := Roots{musicRoot}

	_, err := roots.Browse(otherRoot)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("Browse(%q) error = %v, want ErrOutsideRoots", otherRoot, err)
	}
}

func TestBrowseRejectsTraversalEscape(t *testing.T) {
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	escape := filepath.Join(musicRoot, "..", "other")
	_, err := roots.Browse(escape)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("Browse(%q) error = %v, want ErrOutsideRoots", escape, err)
	}
}

func TestBrowseRejectsSimilarlyPrefixedSibling(t *testing.T) {
	// A root of "/music" must not permit browsing "/music-archive" just
	// because the string "/music" is a prefix of it.
	base := t.TempDir()
	musicRoot := filepath.Join(base, "music")
	siblingRoot := filepath.Join(base, "music-archive")
	if err := os.MkdirAll(musicRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(siblingRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	roots := Roots{musicRoot}
	_, err := roots.Browse(siblingRoot)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("Browse(%q) error = %v, want ErrOutsideRoots", siblingRoot, err)
	}
}

func TestBrowseNonexistentPath(t *testing.T) {
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	_, err := roots.Browse(filepath.Join(musicRoot, "does-not-exist"))
	if err == nil {
		t.Fatal("Browse(nonexistent) error = nil, want error")
	}
	if errors.Is(err, ErrOutsideRoots) {
		t.Fatal("Browse(nonexistent) should fail as a not-found error, not ErrOutsideRoots")
	}
}

func TestBrowseRejectsSymlinkEscapingRoot(t *testing.T) {
	// A symlink that lives inside a configured root but points outside it
	// must not let Browse read (and thus disclose) content outside every
	// configured root, even though its own path string passes the
	// string-prefix containment check.
	musicRoot, otherRoot := setupTree(t)
	roots := Roots{musicRoot}

	escape := filepath.Join(musicRoot, "escape")
	if err := os.Symlink(otherRoot, escape); err != nil {
		t.Fatal(err)
	}

	_, err := roots.Browse(escape)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("Browse(%q) error = %v, want ErrOutsideRoots", escape, err)
	}
}

func TestValidateDirectoryRejectsSymlinkEscapingRoot(t *testing.T) {
	musicRoot, otherRoot := setupTree(t)
	roots := Roots{musicRoot}

	escape := filepath.Join(musicRoot, "escape")
	if err := os.Symlink(otherRoot, escape); err != nil {
		t.Fatal(err)
	}

	_, err := roots.ValidateDirectory(escape)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("ValidateDirectory(%q) error = %v, want ErrOutsideRoots", escape, err)
	}
}

func TestValidateDirectoryAllowsSymlinkStayingWithinRoot(t *testing.T) {
	// A symlink is only a problem when it resolves outside every configured
	// root — one that stays within the same root is legitimate.
	musicRoot, _ := setupTree(t)
	roots := Roots{musicRoot}

	link := filepath.Join(musicRoot, "RockAlias")
	if err := os.Symlink(filepath.Join(musicRoot, "Rock"), link); err != nil {
		t.Fatal(err)
	}

	root, err := roots.ValidateDirectory(link)
	if err != nil {
		t.Fatalf("ValidateDirectory(%q) error = %v, want nil", link, err)
	}
	if root != musicRoot {
		t.Fatalf("ValidateDirectory(%q) root = %q, want %q", link, root, musicRoot)
	}
}
