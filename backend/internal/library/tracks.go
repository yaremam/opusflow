package library

import (
	"context"
	"fmt"
	"log"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// FileError is a single file that an import could not process, recorded
// alongside the import it belongs to rather than aborting the copy.
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// InsertTrack records one successfully-copied audio file, attributing it to
// an artist and an album — finding-or-creating both first (AC-11), so even
// an untagged file (empty Artist/Album) lands under a real "Unknown
// Artist"/"Unknown Album" row rather than being excluded from browsing.
// Satisfies organize.Store.
func (s *Store) InsertTrack(ctx context.Context, t organize.CopiedTrack) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	artistID, err := upsertArtist(ctx, tx, t.Artist)
	if err != nil {
		return err
	}
	albumID, err := upsertAlbum(ctx, tx, t.Album, artistID, t.Year)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tracks (import_id, path, title, artist, album, track_number, year, genre, duration_seconds, artist_id, album_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, t.ImportID, t.Path, t.Title, t.Artist, t.Album, t.TrackNumber, t.Year, t.Genre, t.DurationSeconds, artistID, albumID); err != nil {
		return fmt.Errorf("inserting track: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing track insert: %w", err)
	}

	// Saving embedded artwork happens after commit, outside the
	// transaction — it's disk I/O (resizing), not something that needs
	// the track/artist/album row's atomicity. AC-1: only the first
	// embedded image found for this album wins, so the eventual write is
	// conditioned on the album's art still being open (pending or
	// failed) — a second track's artwork arriving after the first one
	// already landed is a no-op, not an overwrite. A save/record failure
	// here is logged, not returned — the track itself imported fine.
	if s.images != nil && len(t.ArtworkData) > 0 {
		s.saveEmbeddedAlbumArt(ctx, albumID, t.ArtworkData)
	}
	return nil
}

func (s *Store) saveEmbeddedAlbumArt(ctx context.Context, albumID int64, data []byte) {
	// Checked before the decode/resize/encode work below, not just before
	// the final write: most tracks on a tagged album carry the same
	// embedded cover, so without this an album with N tagged tracks would
	// pay the full image-processing cost N times over even though only
	// the first result is ever kept.
	open, err := s.albumArtStillOpen(ctx, albumID)
	if err != nil {
		log.Printf("library: album %d: checking art status: %v", albumID, err)
		return
	}
	if !open {
		return
	}

	thumbURL, fullURL, err := s.images.Save("album", albumID, data)
	if err != nil {
		log.Printf("library: album %d: saving embedded artwork: %v", albumID, err)
		return
	}
	if err := s.SetAlbumArtIfOpen(ctx, albumID, enrich.Found, thumbURL, fullURL); err != nil {
		log.Printf("library: album %d: recording embedded artwork: %v", albumID, err)
	}
}

// albumArtStillOpen reports whether albumID's art is still pending or
// failed (enrich.Store's "not yet settled" definition) — the same test
// SetAlbumArtIfOpen's UPDATE applies, but cheap to run before doing any
// expensive work that a settled album wouldn't need.
func (s *Store) albumArtStillOpen(ctx context.Context, albumID int64) (bool, error) {
	var open bool
	err := s.db.QueryRowContext(ctx, `
		SELECT art_status `+pendingOrFailed+` FROM albums WHERE id = $1
	`, albumID).Scan(&open)
	if err != nil {
		return false, fmt.Errorf("checking album art status: %w", err)
	}
	return open, nil
}

// RecordImportError records that a single file within an import's copy
// could not be processed, without failing the import's overall run.
// Satisfies organize.Store.
func (s *Store) RecordImportError(ctx context.Context, importID int64, path, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO import_errors (import_id, path, error) VALUES ($1, $2, $3)`,
		importID, path, errMsg,
	)
	if err != nil {
		return fmt.Errorf("recording import error: %w", err)
	}
	return nil
}

func (s *Store) listImportErrors(ctx context.Context, importID int64) ([]FileError, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT path, error FROM import_errors WHERE import_id = $1 ORDER BY id`, importID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing import errors: %w", err)
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
