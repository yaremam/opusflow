package organize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/dhowden/tag"
	"github.com/go-flac/flacvorbis/v2"
	goflac "github.com/go-flac/go-flac/v2"

	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// CopiedTrack is one file Copy has successfully placed at its destination,
// ready to be recorded in the catalog. Mirrors the shape scan.Track used for
// the old scan-in-place model, keyed by ImportID instead of DirectoryID.
type CopiedTrack struct {
	ImportID        int64
	Path            string
	Title           string
	Artist          string
	Album           string
	TrackNumber     int
	Year            int
	Genre           string
	DurationSeconds int
	ArtworkData     []byte
}

// Store is what Copy needs from the catalog to record its results and
// progress. Satisfied by library.Store.
type Store interface {
	InsertTrack(ctx context.Context, t CopiedTrack) error
	RecordImportError(ctx context.Context, importID int64, path, message string) error
	UpdateImportProgress(ctx context.Context, importID int64, processed, total int) error
}

// RunSummary reports how a Copy call went. Copy never aborts on a single
// file's failure — that's recorded via Store.RecordImportError and counted
// here instead, mirroring scan.Scanner's per-file tolerance in the old model.
type RunSummary struct {
	FilesProcessed int
	FilesFailed    int
}

// CopyJob adapts Copy to library.Service's Copier interface, mirroring how
// scan.Scanner gave the old model's Service something with a method to
// call rather than a bare function.
type CopyJob struct{}

func (CopyJob) Copy(ctx context.Context, store Store, importID int64, plan Plan) RunSummary {
	return Copy(ctx, store, importID, plan)
}

// Copy executes a confirmed plan: for every track, it copies the source
// file's bytes to Track.DestPath, then writes the plan's (possibly
// user-corrected) Artist/Album/Title/Year/TrackNumber back into the copy's
// own embedded tags — scoped to MP3 and FLAC, the only formats with mature
// Go tag-writing support (TDR 005). Genre and embedded artwork are carried
// through to CopiedTrack from the source file's original tags, since the
// review plan never carries them (AC-9 doesn't ask the reviewer to edit
// either, and threading artwork bytes through the JSON plan would be
// wasteful).
func Copy(ctx context.Context, store Store, importID int64, plan Plan) RunSummary {
	var summary RunSummary
	total := 0
	for _, al := range plan.Albums {
		total += len(al.Tracks)
	}

	processed := 0
	for _, al := range plan.Albums {
		for _, tr := range al.Tracks {
			err := copyTrack(ctx, store, importID, al, tr)
			processed++
			if err != nil {
				summary.FilesFailed++
				log.Printf("library: import %d: %s: %v", importID, tr.SourcePath, err)
				if rerr := store.RecordImportError(ctx, importID, tr.SourcePath, err.Error()); rerr != nil {
					log.Printf("library: import %d: recording error for %s: %v", importID, tr.SourcePath, rerr)
				}
			} else {
				summary.FilesProcessed++
			}
			_ = store.UpdateImportProgress(ctx, importID, processed, total)
		}
	}
	return summary
}

func copyTrack(ctx context.Context, store Store, importID int64, al Album, tr Track) error {
	if err := os.MkdirAll(filepath.Dir(tr.DestPath), 0o755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	if err := copyBytes(tr.SourcePath, tr.DestPath); err != nil {
		return fmt.Errorf("copying file: %w", err)
	}

	genre, artwork, err := readGenreAndArtwork(tr.DestPath)
	if err != nil {
		return fmt.Errorf("reading source tags: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(tr.DestPath))
	switch ext {
	case ".mp3":
		err = writeMP3Tags(tr.DestPath, al, tr)
	case ".flac":
		err = writeFLACTags(tr.DestPath, al, tr)
	}
	if err != nil {
		return fmt.Errorf("writing back tags: %w", err)
	}

	duration := 0
	if parser, ok := scan.DetectFormat(tr.DestPath); ok {
		if f, err := os.Open(tr.DestPath); err == nil {
			if d, err := parser(f); err == nil {
				duration = int(d.Seconds())
			}
			f.Close()
		}
	}

	return store.InsertTrack(ctx, CopiedTrack{
		ImportID:        importID,
		Path:            tr.DestPath,
		Title:           tr.Title,
		Artist:          al.Artist,
		Album:           al.Album,
		TrackNumber:     tr.TrackNumber,
		Year:            al.Year,
		Genre:           genre,
		DurationSeconds: duration,
		ArtworkData:     artwork,
	})
}

func copyBytes(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// readGenreAndArtwork reads the (unmodified, just-copied) destination file's
// original tags for the fields the review plan doesn't carry. A file with
// no tags at all yields both fields blank rather than an error.
func readGenreAndArtwork(path string) (genre string, artwork []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if errors.Is(err, tag.ErrNoTagsFound) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}

	genre = m.Genre()
	if pic := m.Picture(); pic != nil {
		artwork = pic.Data
	}
	return genre, artwork, nil
}

func writeMP3Tags(path string, al Album, tr Track) error {
	tg, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		// Some real-world encoders write a frame whose declared size
		// doesn't reconcile with the tag's own declared byte budget —
		// id3v2's strict frame-by-frame parser refuses to open the file at
		// all (e.g. ErrBodyOverflow). The source file itself can't be
		// fixed, so retry without parsing existing frames and write a
		// fresh tag with just the fields below rather than failing the
		// whole import.
		tg.Close()
		tg, err = id3v2.Open(path, id3v2.Options{Parse: false})
		if err != nil {
			return err
		}
	}
	defer tg.Close()

	tg.SetArtist(al.Artist)
	tg.SetAlbum(al.Album)
	tg.SetTitle(tr.Title)
	tg.SetYear(strconv.Itoa(al.Year))
	tg.AddTextFrame(tg.CommonID("Track number/Position in set"), tg.DefaultEncoding(), strconv.Itoa(tr.TrackNumber))

	return tg.Save()
}

func writeFLACTags(path string, al Album, tr Track) error {
	f, err := goflac.ParseFile(path)
	if err != nil {
		return err
	}
	defer f.Close()

	idx := -1
	for i, m := range f.Meta {
		if m.Type == goflac.VorbisComment {
			idx = i
			break
		}
	}

	var cmt *flacvorbis.MetaDataBlockVorbisComment
	if idx >= 0 {
		cmt, err = flacvorbis.ParseFromMetaDataBlock(*f.Meta[idx])
		if err != nil {
			return err
		}
	} else {
		cmt = flacvorbis.New()
	}

	setVorbisComment(cmt, flacvorbis.FIELD_ARTIST, al.Artist)
	setVorbisComment(cmt, flacvorbis.FIELD_ALBUM, al.Album)
	setVorbisComment(cmt, flacvorbis.FIELD_TITLE, tr.Title)
	setVorbisComment(cmt, flacvorbis.FIELD_DATE, strconv.Itoa(al.Year))
	setVorbisComment(cmt, flacvorbis.FIELD_TRACKNUMBER, strconv.Itoa(tr.TrackNumber))

	block := cmt.Marshal()
	if idx >= 0 {
		f.Meta[idx] = &block
	} else {
		f.Meta = append(f.Meta, &block)
	}

	return f.Save(path)
}

// setVorbisComment overwrites every existing entry for key with a single
// new one — flacvorbis.Add only ever appends, so without this a corrected
// field would end up alongside its stale original rather than replacing it.
func setVorbisComment(cmt *flacvorbis.MetaDataBlockVorbisComment, key, val string) {
	kept := cmt.Comments[:0]
	for _, c := range cmt.Comments {
		if !strings.HasPrefix(strings.ToUpper(c), key+"=") {
			kept = append(kept, c)
		}
	}
	cmt.Comments = kept
	cmt.Add(key, val)
}
