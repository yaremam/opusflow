package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ImportStatus is the lifecycle state of an import run.
type ImportStatus string

const (
	ImportStatusCopying  ImportStatus = "copying"
	ImportStatusComplete ImportStatus = "complete"
	ImportStatusFailed   ImportStatus = "failed"
)

// ErrImportNotFound is returned when an import ID doesn't match any
// recorded import.
var ErrImportNotFound = errors.New("import not found")

// Import is one confirmed import run and its copy progress.
// SourceDescription is a human-readable label for where the files came from
// (the source directory's path, or "Uploaded from device") — imports have
// no natural unique key the way a directory's path once was, so nothing
// here is deduplicated.
type Import struct {
	ID                int64        `json:"id"`
	LibraryID         int64        `json:"libraryId"`
	SourceDescription string       `json:"sourceDescription"`
	Status            ImportStatus `json:"status"`
	FilesProcessed    int          `json:"filesProcessed"`
	FilesTotal        int          `json:"filesTotal"`
	Error             string       `json:"error,omitempty"`
	TrackCount        int          `json:"trackCount"`
	FileErrors        []FileError  `json:"fileErrors"`
	CreatedAt         time.Time    `json:"createdAt"`
}

// CreateImport records a new import run with status "copying", attributed
// to the library it will copy into.
func (s *Store) CreateImport(ctx context.Context, libraryID int64, sourceDescription string) (Import, error) {
	var imp Import
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO imports (library_id, source_description, status)
		VALUES ($1, $2, $3)
		RETURNING id, library_id, source_description, status, files_processed, files_total, error, created_at
	`, libraryID, sourceDescription, ImportStatusCopying).Scan(
		&imp.ID, &imp.LibraryID, &imp.SourceDescription, &imp.Status, &imp.FilesProcessed, &imp.FilesTotal, &imp.Error, &imp.CreatedAt,
	)
	if err != nil {
		return Import{}, fmt.Errorf("inserting import: %w", err)
	}
	imp.FileErrors = []FileError{}
	return imp, nil
}

// importColumnsSQL is the column list scanImport expects, in order.
const importColumnsSQL = `i.id, i.library_id, i.source_description, i.status, i.files_processed, i.files_total, i.error, i.created_at`

// GetImport fetches a single import by ID, including its track count and
// any recorded per-file errors.
func (s *Store) GetImport(ctx context.Context, id int64) (Import, error) {
	imp, err := s.scanImport(s.db.QueryRowContext(ctx, `
		SELECT `+importColumnsSQL+`,
		       COALESCE((SELECT COUNT(*) FROM tracks t WHERE t.import_id = i.id), 0)
		FROM imports i
		WHERE i.id = $1
	`, id))
	if err != nil {
		return Import{}, err
	}

	fileErrors, err := s.listImportErrors(ctx, id)
	if err != nil {
		return Import{}, err
	}
	imp.FileErrors = orEmpty(fileErrors)
	return imp, nil
}

// ListImports returns every recorded import, newest first. File errors
// aren't populated here (an empty slice always) — the list view only needs
// status/progress/track count; GetImport is where a caller looks up one
// import's full error detail.
func (s *Store) ListImports(ctx context.Context) ([]Import, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+importColumnsSQL+`,
		       COALESCE((SELECT COUNT(*) FROM tracks t WHERE t.import_id = i.id), 0)
		FROM imports i
		ORDER BY i.created_at DESC, i.id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing imports: %w", err)
	}
	defer rows.Close()

	imports := []Import{}
	for rows.Next() {
		imp, err := s.scanImport(rows)
		if err != nil {
			return nil, err
		}
		imp.FileErrors = []FileError{}
		imports = append(imports, imp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return imports, nil
}

// UpdateImportProgress records how many of the total files have been
// processed so far. It does not change status. Satisfies organize.Store.
func (s *Store) UpdateImportProgress(ctx context.Context, id int64, processed, total int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE imports SET files_processed = $1, files_total = $2 WHERE id = $3`,
		processed, total, id,
	)
	if err != nil {
		return fmt.Errorf("updating import progress: %w", err)
	}
	return nil
}

// MarkImportComplete marks an import's copy as finished (per-file errors,
// if any, don't prevent this).
func (s *Store) MarkImportComplete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE imports SET status = $1 WHERE id = $2`, ImportStatusComplete, id,
	)
	if err != nil {
		return fmt.Errorf("marking import complete: %w", err)
	}
	return nil
}

// MarkImportFailed marks an import as unable to proceed at all, recording
// why.
func (s *Store) MarkImportFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE imports SET status = $1, error = $2 WHERE id = $3`, ImportStatusFailed, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("marking import failed: %w", err)
	}
	return nil
}

// scanImport scans one row produced by a query built on importColumnsSQL —
// the 7 base columns, then track count as the 8th — into an Import.
func (s *Store) scanImport(row rowScanner) (Import, error) {
	var imp Import
	err := row.Scan(
		&imp.ID, &imp.LibraryID, &imp.SourceDescription, &imp.Status, &imp.FilesProcessed, &imp.FilesTotal, &imp.Error, &imp.CreatedAt, &imp.TrackCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Import{}, ErrImportNotFound
	}
	if err != nil {
		return Import{}, fmt.Errorf("scanning import: %w", err)
	}
	return imp, nil
}
