package organize

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/dhowden/tag"

	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// Track is one audio file found under a source directory, with its
// destination computed against the library root it would be organized
// into. A blank Title or zero TrackNumber means the source file's tags
// didn't provide one — left that way rather than guessed (TDR 005 AC-7);
// DestPath still gets computed (see destPath), just visibly incomplete.
type Track struct {
	SourcePath  string `json:"sourcePath"`
	TrackNumber int    `json:"trackNumber"`
	Title       string `json:"title"`
	DestPath    string `json:"destPath"`
	Conflict    bool   `json:"conflict"`

	// Overwrite is set by the reviewer to explicitly accept clobbering the
	// file already at DestPath (TDR 005's block-until-resolved conflict
	// handling — never defaulted true). Ignored by BuildPlan; only read by
	// Validate/Confirm.
	Overwrite bool `json:"overwrite"`
}

// Album is one detected (Artist, Album) group within a source directory,
// carrying every track BuildPlan attributed to it.
type Album struct {
	Artist string  `json:"artist"`
	Album  string  `json:"album"`
	Year   int     `json:"year"`
	Tracks []Track `json:"tracks"`
}

// Plan is the full result of reading a source directory — nothing has been
// copied yet. The caller (library.Service) hands this to the review screen
// as-is; confirming re-derives it from the edited fields rather than
// trusting this snapshot; see Confirm.
type Plan struct {
	Albums []Album `json:"albums"`
}

// albumKey groups tracks by their raw (possibly blank) Artist/Album tag
// values — two untagged files in the same source directory land in the
// same blank-fields group rather than each getting their own, since
// nothing about them says otherwise.
type albumKey struct {
	artist string
	album  string
}

// BuildPlan reads tags from every recognized audio file under sourceDir
// (recursively) and groups them into per-album plans, each track's
// destination computed against libraryRoot. A file that can't be opened or
// whose tags are malformed is skipped rather than failing the whole plan —
// same per-file tolerance scan.Scanner uses, since a single bad file
// shouldn't block reviewing everything else that's fine.
func BuildPlan(libraryRoot, sourceDir string) (Plan, error) {
	groups := map[albumKey]*Album{}
	var order []albumKey

	err := filepath.WalkDir(sourceDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == sourceDir {
				return err
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if _, ok := scan.DetectFormat(d.Name()); !ok {
			return nil
		}

		artist, album, title, trackNumber, year, err := readTrackTags(p)
		if err != nil {
			return nil
		}

		key := albumKey{artist, album}
		grp, ok := groups[key]
		if !ok {
			grp = &Album{Artist: artist, Album: album, Year: year}
			groups[key] = grp
			order = append(order, key)
		}

		dest := destPath(libraryRoot, artist, album, year, trackNumber, title, filepath.Ext(p))

		grp.Tracks = append(grp.Tracks, Track{
			SourcePath:  p,
			TrackNumber: trackNumber,
			Title:       title,
			DestPath:    dest,
			Conflict:    destExists(dest),
		})
		return nil
	})
	if err != nil {
		return Plan{}, fmt.Errorf("reading source directory: %w", err)
	}

	plan := Plan{}
	for _, key := range order {
		grp := groups[key]
		sort.SliceStable(grp.Tracks, func(i, j int) bool { return grp.Tracks[i].TrackNumber < grp.Tracks[j].TrackNumber })
		plan.Albums = append(plan.Albums, *grp)
	}
	return plan, nil
}

// readTrackTags reads raw tag fields from the file at path, leaving every
// field at its zero value when tags don't provide one. Deliberately
// distinct from scan.ExtractTags: that function backfills Title from the
// filename when tags don't carry one, a convention that made sense for the
// old scan-in-place model (no review step to ever catch a blank field).
// organize's review screen is exactly where a blank field belongs (AC-7),
// so no fallback/guess happens here.
func readTrackTags(path string) (artist, album, title string, trackNumber, year int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", "", 0, 0, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if errors.Is(err, tag.ErrNoTagsFound) {
		return "", "", "", 0, 0, nil
	}
	if err != nil {
		return "", "", "", 0, 0, err
	}

	trackNumber, _ = m.Track()
	return fixCyrillicMojibake(m.Artist()), fixCyrillicMojibake(m.Album()), fixCyrillicMojibake(m.Title()), trackNumber, m.Year(), nil
}
