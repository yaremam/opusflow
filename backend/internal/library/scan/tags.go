package scan

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
)

// Tags is the metadata extracted from one audio file.
type Tags struct {
	Title       string
	Artist      string
	Album       string
	TrackNumber int
	Year        int
	Genre       string
}

// ExtractTags reads title/artist/album/track/year/genre tags from f, an
// already-open file at path (path is only used for the filename fallback
// below and in error messages — the caller owns f's lifecycle, since it's
// also reused for duration parsing).
//
// If the file carries no usable tags — either because dhowden/tag can't
// identify any tag format at all (tag.ErrNoTagsFound, the common case for
// WAV, which rarely carries tags) or because it found a container but every
// field came back empty — that is not treated as a failure: Tags is
// returned with Title falling back to the filename (extension stripped)
// and a nil error, per AC-6. Only tag data that's present but malformed is
// returned as an error, for the caller to record as a per-file scan
// failure (AC-7).
func ExtractTags(path string, f *os.File) (Tags, error) {
	fallbackTitle := filenameWithoutExt(path)

	m, err := tag.ReadFrom(f)
	if errors.Is(err, tag.ErrNoTagsFound) {
		return Tags{Title: fallbackTitle}, nil
	}
	if err != nil {
		return Tags{}, fmt.Errorf("reading tags: %w", err)
	}

	title := m.Title()
	if title == "" {
		title = fallbackTitle
	}
	trackNumber, _ := m.Track()

	return Tags{
		Title:       title,
		Artist:      m.Artist(),
		Album:       m.Album(),
		TrackNumber: trackNumber,
		Year:        m.Year(),
		Genre:       m.Genre(),
	}, nil
}

func filenameWithoutExt(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}
