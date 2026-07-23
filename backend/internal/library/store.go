package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Status is the lifecycle state of a registered library directory.
type Status string

const (
	StatusScanning Status = "scanning"
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
)

// postgresUniqueViolation is the SQLSTATE code for a unique constraint
// violation. See https://www.postgresql.org/docs/current/errcodes-appendix.html
const postgresUniqueViolation = "23505"

// ErrDuplicateDirectory is returned by AddDirectory when the given path is
// already registered.
var ErrDuplicateDirectory = errors.New("directory already registered")

// ErrDirectoryNotFound is returned when a directory ID doesn't match any
// registered directory.
var ErrDirectoryNotFound = errors.New("directory not found")

// Directory is a registered library directory and its scan state.
type Directory struct {
	ID             int64       `json:"id"`
	Root           string      `json:"root"`
	Path           string      `json:"path"`
	Status         Status      `json:"status"`
	FilesProcessed int         `json:"filesProcessed"`
	FilesTotal     int         `json:"filesTotal"`
	Error          string      `json:"error,omitempty"`
	TrackCount     int         `json:"trackCount"`
	FileErrors     []FileError `json:"fileErrors"`
	CreatedAt      time.Time   `json:"createdAt"`
}

// Store persists library directories, their imported tracks, and any
// per-file scan errors.
type Store struct {
	db *sql.DB
}

// NewStore wraps an already-migrated Postgres connection.
func NewStore(conn *sql.DB) *Store {
	return &Store{db: conn}
}

// AddDirectory registers a new library directory with status "scanning".
// It returns ErrDuplicateDirectory if path is already registered.
func (s *Store) AddDirectory(ctx context.Context, root, path string) (Directory, error) {
	var d Directory
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO library_directories (root, path, status)
		VALUES ($1, $2, $3)
		RETURNING id, root, path, status, files_processed, files_total, error, created_at
	`, root, path, StatusScanning).Scan(
		&d.ID, &d.Root, &d.Path, &d.Status, &d.FilesProcessed, &d.FilesTotal, &d.Error, &d.CreatedAt,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == postgresUniqueViolation {
			return Directory{}, ErrDuplicateDirectory
		}
		return Directory{}, fmt.Errorf("inserting directory: %w", err)
	}
	d.FileErrors = []FileError{}
	return d, nil
}

// directoryColumnsSQL is the column list shared by every query that scans a
// Directory via scanDirectory: id, root, path, status, files_processed,
// files_total, error, created_at, in the order scanDirectory expects. Each
// caller appends its own way of computing the 9th column, track count,
// since GetDirectory and ListDirectories compute it differently (a
// correlated subquery vs. a pre-aggregated join) — that expression isn't
// shared, only the 8 columns that are identical either way.
const directoryColumnsSQL = `d.id, d.root, d.path, d.status, d.files_processed, d.files_total, d.error, d.created_at`

// GetDirectory fetches a single directory by ID, including its track count
// and any recorded per-file errors.
func (s *Store) GetDirectory(ctx context.Context, id int64) (Directory, error) {
	d, err := s.scanDirectory(s.db.QueryRowContext(ctx, `
		SELECT `+directoryColumnsSQL+`,
		       COALESCE((SELECT COUNT(*) FROM tracks t WHERE t.directory_id = d.id), 0)
		FROM library_directories d
		WHERE d.id = $1
	`, id))
	if err != nil {
		return Directory{}, err
	}

	fileErrors, err := s.listFileErrors(ctx, id)
	if err != nil {
		return Directory{}, err
	}
	d.FileErrors = orEmpty(fileErrors)
	return d, nil
}

// ListDirectories returns every registered directory, oldest first, each
// with its track count and per-file errors populated.
func (s *Store) ListDirectories(ctx context.Context) ([]Directory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+directoryColumnsSQL+`,
		       COALESCE(t.track_count, 0)
		FROM library_directories d
		LEFT JOIN (
			SELECT directory_id, COUNT(*) AS track_count FROM tracks GROUP BY directory_id
		) t ON t.directory_id = d.id
		ORDER BY d.created_at, d.id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing directories: %w", err)
	}
	defer rows.Close()

	var dirs []Directory
	for rows.Next() {
		d, err := s.scanDirectory(rows)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	allErrors, err := s.listAllFileErrors(ctx)
	if err != nil {
		return nil, err
	}
	for i := range dirs {
		dirs[i].FileErrors = orEmpty(allErrors[dirs[i].ID])
	}

	return dirs, nil
}

// orEmpty returns a non-nil empty slice in place of nil, so JSON responses
// use "[]" rather than "null" for an absence of file errors.
func orEmpty(errs []FileError) []FileError {
	if errs == nil {
		return []FileError{}
	}
	return errs
}

// RemoveDirectory deletes a directory and, via ON DELETE CASCADE, all of
// its tracks and recorded file errors.
func (s *Store) RemoveDirectory(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM library_directories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting directory: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrDirectoryNotFound
	}
	return nil
}

// UpdateProgress records how many of the total matching audio files have
// been processed so far. It does not change status.
func (s *Store) UpdateProgress(ctx context.Context, id int64, processed, total int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE library_directories SET files_processed = $1, files_total = $2 WHERE id = $3`,
		processed, total, id,
	)
	if err != nil {
		return fmt.Errorf("updating progress: %w", err)
	}
	return nil
}

// MarkComplete marks a directory's scan as finished successfully (per-file
// errors, if any, don't prevent this).
func (s *Store) MarkComplete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE library_directories SET status = $1 WHERE id = $2`, StatusComplete, id,
	)
	if err != nil {
		return fmt.Errorf("marking directory complete: %w", err)
	}
	return nil
}

// MarkFailed marks a directory's scan as unable to proceed at all (e.g. the
// directory itself became inaccessible), recording why.
func (s *Store) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE library_directories SET status = $1, error = $2 WHERE id = $3`, StatusFailed, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("marking directory failed: %w", err)
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row (GetDirectory's single-row
// query) and *sql.Rows (ListDirectories' per-row loop), so scanDirectory
// serves both instead of each query hand-rolling its own Scan call.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanDirectory scans one row produced by a query built on
// directoryColumnsSQL — the 8 base columns, then track count as the 9th —
// into a Directory.
func (s *Store) scanDirectory(row rowScanner) (Directory, error) {
	var d Directory
	err := row.Scan(
		&d.ID, &d.Root, &d.Path, &d.Status, &d.FilesProcessed, &d.FilesTotal, &d.Error, &d.CreatedAt, &d.TrackCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Directory{}, ErrDirectoryNotFound
	}
	if err != nil {
		return Directory{}, fmt.Errorf("scanning directory: %w", err)
	}
	return d, nil
}
