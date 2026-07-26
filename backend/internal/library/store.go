package library

import (
	"database/sql"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// postgresUniqueViolation is the SQLSTATE code for a unique constraint
// violation. See https://www.postgresql.org/docs/current/errcodes-appendix.html
const postgresUniqueViolation = "23505"

// Store persists imports, the tracks they copy in, and the artist/album
// catalog those tracks populate.
type Store struct {
	db     *sql.DB
	images *enrich.ImageStore
}

// NewStore wraps an already-migrated Postgres connection. images is
// optional (nil is fine, e.g. in tests that don't exercise artwork) — set
// it via SetImages to enable saving embedded cover art at copy time (AC-1);
// until then, InsertTrack silently skips any artwork a track carries rather
// than failing the import over it.
func NewStore(conn *sql.DB) *Store {
	return &Store{db: conn}
}

// SetImages wires up embedded-artwork saving (AC-1) — called once at
// startup alongside NewStore, kept separate so every existing NewStore
// callsite (tests included) doesn't need an ARTWORK_DIR to construct a
// Store.
func (s *Store) SetImages(images *enrich.ImageStore) {
	s.images = images
}

// orEmpty returns a non-nil empty slice in place of nil, so JSON responses
// use "[]" rather than "null" for an absence of file errors.
func orEmpty(errs []FileError) []FileError {
	if errs == nil {
		return []FileError{}
	}
	return errs
}

// rowScanner is satisfied by both *sql.Row (a single-row query) and
// *sql.Rows (a per-row loop), so a scan helper can serve both instead of
// each query hand-rolling its own Scan call.
type rowScanner interface {
	Scan(dest ...any) error
}
