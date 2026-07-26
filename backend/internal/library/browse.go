// Package library implements the household music catalog: browsing the
// filesystem to choose an import source or a new library's root, and the
// artist/album/song catalog those imports populate.
package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNotADirectory is returned when a requested path exists but names a
// regular file rather than a regular directory.
var ErrNotADirectory = errors.New("path is not a directory")

// ErrOutsideRoot is returned when a path falls outside the browse root
// configured via DATA_DIR. There is no such root by default (TDR 006) — an
// unconfigured root means WithinRoot never rejects anything.
var ErrOutsideRoot = errors.New("path is outside the configured data directory")

// WithinRoot reports whether path is root itself or somewhere underneath
// it, comparing cleaned path segments (not raw byte prefixes, so a sibling
// like /data-other is never mistaken for being inside /data). An empty root
// means no restriction is configured, so every path passes.
func WithinRoot(path, root string) error {
	if root == "" {
		return nil
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s: %w", path, ErrOutsideRoot)
	}
	return nil
}

// Entry is one subdirectory returned by Browse.
type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Browse lists the immediate subdirectories of path. There is no configured
// allowlist to check path against (TDR 006) — the container's own mounts
// are the real boundary on what's reachable at all, so browsing itself is
// unrestricted from the filesystem root down.
func Browse(path string) ([]Entry, error) {
	clean := filepath.Clean(path)

	items, err := os.ReadDir(clean)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", clean, err)
	}

	var entries []Entry
	for _, item := range items {
		if !item.IsDir() {
			continue
		}
		entries = append(entries, Entry{
			Name: item.Name(),
			Path: filepath.Join(clean, item.Name()),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

// ValidateDirectory confirms path names an existing directory — the one
// check left for a library's root once there's no allowlist to validate it
// against (TDR 006). Returns a wrapped os.Stat error (unwrappable to
// fs.ErrNotExist, among others) if it can't be confirmed as a directory.
func ValidateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("checking directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s: %w", path, ErrNotADirectory)
	}
	return nil
}

// ErrInvalidFolderName is returned when CreateFolder's name isn't a single
// path segment — no separators, and no "." or ".." parent references.
var ErrInvalidFolderName = errors.New("invalid folder name")

// CreateFolder makes a new subdirectory named name directly inside parent
// (never nested — name must be a single path segment) so a library's root
// no longer has to already exist on the host before it can be picked.
// Idempotent: an existing directory with that name is left as-is and
// returned rather than treated as an error, since the most likely reason a
// caller hits that is re-selecting a folder they already made.
func CreateFolder(parent, name string) (Entry, error) {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) {
		return Entry{}, fmt.Errorf("%q: %w", name, ErrInvalidFolderName)
	}
	if err := ValidateDirectory(parent); err != nil {
		return Entry{}, err
	}

	path := filepath.Join(filepath.Clean(parent), name)
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return Entry{}, fmt.Errorf("creating directory: %w", err)
	}
	if err := ValidateDirectory(path); err != nil {
		return Entry{}, err
	}
	return Entry{Name: name, Path: path}, nil
}
