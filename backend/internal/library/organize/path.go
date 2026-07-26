// Package organize builds and executes an import plan (TDR 005): reading
// tags from a source directory, grouping files into per-album plans with
// computed destination paths, and — once a plan is confirmed — copying
// each file into LIBRARY_ROOT's canonical <Artist>/<Year>.<Album>/<NN>.<Title>
// layout.
package organize

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// illegalPathChars strips characters that are illegal (or awkward — colon
// and pipe cause real problems on exFAT/NTFS shares a NAS might expose
// this over) in a filesystem path segment, plus control characters.
var illegalPathChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// sanitizeSegment makes s safe to use as one path segment (a folder or file
// name, not a full path) by stripping illegal characters and trimming
// leading/trailing dots and whitespace — trailing dots are rejected outright
// by Windows-influenced filesystems, and a leading dot creates a hidden
// file on Unix, neither of which anyone importing an album expects.
// Deliberately does not touch an empty string into a placeholder — a blank
// field stays visibly blank in the resulting path (TDR 005 AC-7); it is the
// review screen's job to flag that, not this function's to hide it.
func sanitizeSegment(s string) string {
	s = illegalPathChars.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".")
	return s
}

// destPath computes the canonical destination for one track:
// <libraryRoot>/<Artist>/<Year>.<Album>/<NN>.<Title><ext>, NN being
// trackNumber zero-padded to two digits. Any blank/zero field still
// produces a syntactically valid (if visibly incomplete) path rather than
// erroring — see sanitizeSegment.
func destPath(libraryRoot, artist, album string, year, trackNumber int, title, ext string) string {
	artistSeg := sanitizeSegment(artist)
	albumSeg := fmt.Sprintf("%d.%s", year, sanitizeSegment(album))
	fileSeg := fmt.Sprintf("%02d.%s%s", trackNumber, sanitizeSegment(title), ext)
	return filepath.Join(libraryRoot, artistSeg, albumSeg, fileSeg)
}

// destExists reports whether path already exists on disk, treating any
// stat error other than "not found" as "doesn't exist" too — Validate
// errs on the side of allowing a copy attempt (which will surface its own,
// clearer error) rather than blocking one over e.g. a permissions quirk on
// an ancestor directory that hasn't been created yet.
func destExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
