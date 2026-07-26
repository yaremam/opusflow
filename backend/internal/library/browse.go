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
)

// ErrNotADirectory is returned when a requested path exists but names a
// regular file rather than a directory.
var ErrNotADirectory = errors.New("path is not a directory")

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
