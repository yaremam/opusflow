package library

import (
	"context"
	"fmt"

	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// FileError is a single file that a scan could not process, recorded
// alongside the directory it belongs to rather than aborting the scan.
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// InsertTrack records one successfully-scanned audio file.
func (s *Store) InsertTrack(ctx context.Context, t scan.Track) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tracks (directory_id, path, title, artist, album, track_number, year, genre, duration_seconds)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, t.DirectoryID, t.Path, t.Title, t.Artist, t.Album, t.TrackNumber, t.Year, t.Genre, t.DurationSeconds)
	if err != nil {
		return fmt.Errorf("inserting track: %w", err)
	}
	return nil
}

// RecordFileError records that a single file within a directory's scan
// could not be processed, without failing the directory's overall scan.
func (s *Store) RecordFileError(ctx context.Context, directoryID int64, path, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO library_scan_errors (directory_id, path, error) VALUES ($1, $2, $3)`,
		directoryID, path, errMsg,
	)
	if err != nil {
		return fmt.Errorf("recording file error: %w", err)
	}
	return nil
}

func (s *Store) listFileErrors(ctx context.Context, directoryID int64) ([]FileError, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, error FROM library_scan_errors WHERE directory_id = $1 ORDER BY id`, directoryID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing file errors: %w", err)
	}
	defer rows.Close()

	var errs []FileError
	for rows.Next() {
		var fe FileError
		if err := rows.Scan(&fe.Path, &fe.Error); err != nil {
			return nil, err
		}
		errs = append(errs, fe)
	}
	return errs, rows.Err()
}

func (s *Store) listAllFileErrors(ctx context.Context) (map[int64][]FileError, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT directory_id, path, error FROM library_scan_errors ORDER BY directory_id, id`,
	)
	if err != nil {
		return nil, fmt.Errorf("listing file errors: %w", err)
	}
	defer rows.Close()

	byDir := make(map[int64][]FileError)
	for rows.Next() {
		var dirID int64
		var fe FileError
		if err := rows.Scan(&dirID, &fe.Path, &fe.Error); err != nil {
			return nil, err
		}
		byDir[dirID] = append(byDir[dirID], fe)
	}
	return byDir, rows.Err()
}
