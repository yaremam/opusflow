package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrLibraryNotFound is returned when a library ID doesn't match any
// recorded library.
var ErrLibraryNotFound = errors.New("library not found")

// Library is a named root folder opusflow organizes imports into (TDR 006).
// Created and managed entirely within the app — there is no environment
// variable equivalent anymore.
type Library struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	RootPath   string    `json:"rootPath"`
	TrackCount int       `json:"trackCount"`
	CreatedAt  time.Time `json:"createdAt"`
}

// CreateLibrary records a new library. rootPath must already name an
// existing directory — there's no configured allowlist to validate it
// against anymore (TDR 006), so this is the one check left.
func (s *Store) CreateLibrary(ctx context.Context, name, rootPath string) (Library, error) {
	if err := ValidateDirectory(rootPath); err != nil {
		return Library{}, err
	}

	var lib Library
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO libraries (name, root_path)
		VALUES ($1, $2)
		RETURNING id, name, root_path, created_at
	`, name, rootPath).Scan(&lib.ID, &lib.Name, &lib.RootPath, &lib.CreatedAt)
	if err != nil {
		return Library{}, fmt.Errorf("inserting library: %w", err)
	}
	return lib, nil
}

// libraryTrackCountSQL counts tracks attributed to a library via its
// imports — tracks don't carry a library_id of their own; imports.library_id
// is the only place that link is recorded (catalog browsing stays
// library-agnostic per TDR 006 AC-2).
const libraryTrackCountSQL = `COALESCE((SELECT COUNT(*) FROM tracks t JOIN imports i ON i.id = t.import_id WHERE i.library_id = l.id), 0)`

// ListLibraries returns every library, oldest first.
func (s *Store) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.id, l.name, l.root_path, l.created_at, `+libraryTrackCountSQL+`
		FROM libraries l
		ORDER BY l.created_at, l.id
	`)
	if err != nil {
		return nil, fmt.Errorf("listing libraries: %w", err)
	}
	defer rows.Close()

	libs := []Library{}
	for rows.Next() {
		var lib Library
		if err := rows.Scan(&lib.ID, &lib.Name, &lib.RootPath, &lib.CreatedAt, &lib.TrackCount); err != nil {
			return nil, fmt.Errorf("scanning library: %w", err)
		}
		libs = append(libs, lib)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return libs, nil
}

// GetLibrary fetches a single library by ID.
func (s *Store) GetLibrary(ctx context.Context, id int64) (Library, error) {
	var lib Library
	err := s.db.QueryRowContext(ctx, `
		SELECT l.id, l.name, l.root_path, l.created_at, `+libraryTrackCountSQL+`
		FROM libraries l
		WHERE l.id = $1
	`, id).Scan(&lib.ID, &lib.Name, &lib.RootPath, &lib.CreatedAt, &lib.TrackCount)
	if errors.Is(err, sql.ErrNoRows) {
		return Library{}, ErrLibraryNotFound
	}
	if err != nil {
		return Library{}, fmt.Errorf("getting library: %w", err)
	}
	return lib, nil
}
