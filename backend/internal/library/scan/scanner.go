package scan

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"time"
)

// progressUpdateInterval throttles how often Scan writes progress to the
// store — every file would mean one UPDATE per track on large libraries,
// far more often than the web UI's poll interval can even observe.
const progressUpdateInterval = 20

// terminalWriteRetries/terminalWriteBackoff bound the effort Scan puts into
// making MarkComplete/MarkFailed land. Progress and per-file-error writes
// are informational and safe to drop if they fail — the next successful
// write supersedes them. A terminal write is not: if it never lands, the
// directory is stuck at "scanning" forever with nothing else that will ever
// revisit it, so it's worth a few retries against a transient DB error
// before giving up.
const (
	terminalWriteRetries = 3
	terminalWriteBackoff = 200 * time.Millisecond
)

// ProgressStore is the persistence a Scanner needs while it works: a place
// to report progress, store successfully-scanned tracks, record per-file
// errors, and report the eventual outcome. library.Store satisfies this.
type ProgressStore interface {
	UpdateProgress(ctx context.Context, directoryID int64, processed, total int) error
	InsertTrack(ctx context.Context, t Track) error
	RecordFileError(ctx context.Context, directoryID int64, path, errMsg string) error
	MarkComplete(ctx context.Context, directoryID int64) error
	MarkFailed(ctx context.Context, directoryID int64, errMsg string) error
}

// Scanner recursively imports a library directory's audio files.
type Scanner struct {
	store ProgressStore
}

// NewScanner builds a Scanner backed by store.
func NewScanner(store ProgressStore) *Scanner {
	return &Scanner{store: store}
}

// audioFile is one recognized-format file found while walking a directory,
// paired with the parser that computes its duration.
type audioFile struct {
	path          string
	parseDuration DurationParser
}

// Scan recursively walks path, importing every recognized audio file it can
// process as a track and recording per-file errors for the rest (AC-7),
// then marks the directory complete. If path itself can't be read at all,
// it marks the directory failed instead (AC-9) — a job-level failure,
// distinct from tolerated per-file/per-subdirectory errors.
//
// ctx doubles as a cancellation signal: if the caller cancels it (because,
// for example, the directory was removed while the scan was still
// running), Scan stops as soon as it notices and writes no further state —
// there may be nothing left in the store to write it to.
func (s *Scanner) Scan(ctx context.Context, directoryID int64, path string) {
	files, err := s.collectAudioFiles(ctx, directoryID, path)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		s.markFailed(ctx, directoryID, fmt.Sprintf("could not read directory: %v", err))
		return
	}

	total := len(files)
	s.updateProgress(ctx, directoryID, 0, total)

	processed := 0
	for _, af := range files {
		if ctx.Err() != nil {
			return
		}
		s.importFile(ctx, directoryID, af.path, af.parseDuration)
		processed++
		if processed%progressUpdateInterval == 0 {
			s.updateProgress(ctx, directoryID, processed, total)
		}
	}
	if ctx.Err() != nil {
		return
	}

	s.updateProgress(ctx, directoryID, processed, total)
	s.markComplete(ctx, directoryID)
}

// collectAudioFiles walks path once, recording a per-file error (AC-7) for
// every entry it can't read and returning every recognized audio file it
// found. This is the single source of truth for both the "total" progress
// count and the files Scan goes on to import, so the two can never
// disagree — a directory Scan can't fully enumerate is reflected the same
// way whether you're looking at the initial count or the final tally.
func (s *Scanner) collectAudioFiles(ctx context.Context, directoryID int64, path string) ([]audioFile, error) {
	var files []audioFile
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return fs.SkipAll
		}
		if err != nil {
			if p == path {
				return err
			}
			s.recordFileError(ctx, directoryID, p, err.Error())
			return nil
		}
		if d.IsDir() {
			return nil
		}

		if parse, ok := DetectFormat(d.Name()); ok {
			files = append(files, audioFile{path: p, parseDuration: parse})
		}
		return nil
	})
	return files, err
}

func (s *Scanner) importFile(ctx context.Context, directoryID int64, path string, parseDuration DurationParser) {
	f, err := os.Open(path)
	if err != nil {
		s.recordFileError(ctx, directoryID, path, fmt.Errorf("opening file: %w", err).Error())
		return
	}
	defer f.Close()

	tags, err := ExtractTags(path, f)
	if err != nil {
		s.recordFileError(ctx, directoryID, path, err.Error())
		return
	}

	// Duration is supplementary: a malformed file shouldn't turn an
	// otherwise-good file into a per-file scan error. Reuses the same open
	// file tags were just read from rather than opening path a second time.
	dur, _ := parseDuration(f)

	track := Track{
		DirectoryID:     directoryID,
		Path:            path,
		Title:           tags.Title,
		Artist:          tags.Artist,
		Album:           tags.Album,
		TrackNumber:     tags.TrackNumber,
		Year:            tags.Year,
		Genre:           tags.Genre,
		DurationSeconds: int(dur / time.Second),
	}
	if err := s.store.InsertTrack(ctx, track); err != nil {
		s.recordFileError(ctx, directoryID, path, err.Error())
	}
}

// recordFileError, updateProgress, markComplete, and markFailed wrap their
// ProgressStore calls with logging (and, for the two terminal writes,
// retries): Scan runs unattended in a background goroutine, so a caller has
// no way to observe or react to a store error itself.
func (s *Scanner) recordFileError(ctx context.Context, directoryID int64, path, msg string) {
	if err := s.store.RecordFileError(ctx, directoryID, path, msg); err != nil {
		log.Printf("library scan: directory %d: recording file error for %s: %v", directoryID, path, err)
	}
}

func (s *Scanner) updateProgress(ctx context.Context, directoryID int64, processed, total int) {
	if err := s.store.UpdateProgress(ctx, directoryID, processed, total); err != nil {
		log.Printf("library scan: directory %d: recording progress: %v", directoryID, err)
	}
}

func (s *Scanner) markComplete(ctx context.Context, directoryID int64) {
	err := retry(terminalWriteRetries, terminalWriteBackoff, func() error {
		return s.store.MarkComplete(ctx, directoryID)
	})
	if err != nil {
		log.Printf("library scan: directory %d: marking complete: %v", directoryID, err)
	}
}

func (s *Scanner) markFailed(ctx context.Context, directoryID int64, msg string) {
	err := retry(terminalWriteRetries, terminalWriteBackoff, func() error {
		return s.store.MarkFailed(ctx, directoryID, msg)
	})
	if err != nil {
		log.Printf("library scan: directory %d: marking failed (%s): %v", directoryID, msg, err)
	}
}

// retry calls fn up to attempts times, pausing backoff between tries,
// returning the last error if none succeed.
func retry(attempts int, backoff time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < attempts-1 {
			time.Sleep(backoff)
		}
	}
	return err
}
