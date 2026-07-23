// Package library implements filesystem access scoped to the host
// directories opusflow is allowed to treat as library sources.
package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Roots is the set of filesystem paths opusflow is allowed to browse and
// register library directories under, configured via the LIBRARY_ROOTS env
// var — one entry per Docker volume mount.
type Roots []string

// ParseRoots parses a comma-separated LIBRARY_ROOTS value into a Roots set,
// skipping empty entries, trimming whitespace, and cleaning each path.
func ParseRoots(env string) Roots {
	var roots Roots
	for _, p := range strings.Split(env, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		roots = append(roots, filepath.Clean(p))
	}
	return roots
}

// ErrOutsideRoots is returned when a requested path does not resolve to a
// configured root or a descendant of one.
var ErrOutsideRoots = errors.New("path is outside configured library roots")

// ErrNotADirectory is returned when a requested path exists but names a
// regular file rather than a directory.
var ErrNotADirectory = errors.New("path is not a directory")

// Entry is one subdirectory returned by Browse.
type Entry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Browse lists the immediate subdirectories of path. path must be exactly
// one of the configured roots, or nested underneath one — anything else
// (including paths that only share a string prefix with a root, like
// "/music-archive" under root "/music") is rejected with ErrOutsideRoots.
func (r Roots) Browse(path string) ([]Entry, error) {
	clean := filepath.Clean(path)
	if _, ok := r.RootFor(clean); !ok {
		return nil, ErrOutsideRoots
	}

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

// RootFor reports which configured root path is exactly, or is nested
// underneath, and whether it's safe to read from disk. This is one atomic
// check, not two: a path must pass both a string-prefix match against the
// configured roots (rejecting a path like "/music-archive" that only
// shares a string prefix with root "/music") and, if it can be resolved
// (i.e. it exists), a symlink-resolved match too — a symlink that lives
// inside a configured root but points outside it would pass the string
// check alone while still letting a caller's subsequent os.ReadDir/os.Stat
// read arbitrary filesystem content. There is deliberately no way to get
// only one half of this check.
//
// A path that doesn't exist yet is still reported as contained here (its
// symlinks simply can't be resolved to prove otherwise) — that's a
// not-found error the caller's own existence check (os.Stat/os.ReadDir)
// produces separately, not a containment violation.
func (r Roots) RootFor(path string) (string, bool) {
	root, ok := r.stringRootFor(path)
	if !ok {
		return "", false
	}
	if r.escapesRootsViaSymlink(path) {
		return "", false
	}
	return root, true
}

// stringRootFor reports which configured root path's string is exactly, or
// is nested underneath, checking only the path string itself.
func (r Roots) stringRootFor(path string) (string, bool) {
	for _, root := range r {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return root, true
		}
	}
	return "", false
}

// escapesRootsViaSymlink reports whether path's fully symlink-resolved form
// falls outside every configured root, even though path's own string
// passed stringRootFor. It only returns true when path can actually be
// resolved (i.e. exists) and resolves outside the roots — see RootFor.
func (r Roots) escapesRootsViaSymlink(path string) bool {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	for _, root := range r {
		realRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		if real == realRoot || strings.HasPrefix(real, realRoot+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

// ValidateDirectory reports the configured root that path (already cleaned)
// is nested under, after confirming path names an existing directory.
// Returns ErrOutsideRoots if path escapes the configured roots (including
// via a symlink that resolves outside them), or a wrapped os.Stat error
// (unwrappable to fs.ErrNotExist, among others) if it can't be confirmed as
// a directory.
func (r Roots) ValidateDirectory(path string) (string, error) {
	root, ok := r.RootFor(path)
	if !ok {
		return "", ErrOutsideRoots
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("checking directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s: %w", path, ErrNotADirectory)
	}
	return root, nil
}
