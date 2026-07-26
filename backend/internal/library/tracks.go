package library

import (
	"context"
	"fmt"
	"log"

	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

// FileError is a single file that a scan could not process, recorded
// alongside the directory it belongs to rather than aborting the scan.
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// InsertTrack records one successfully-scanned audio file, attributing it
// to an artist and an album — finding-or-creating both first (AC-11), so
// even an untagged file (empty Artist/Album) lands under a real "Unknown
// Artist"/"Unknown Album" row rather than being excluded from browsing.
func (s *Store) InsertTrack(ctx context.Context, t scan.Track) error {
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
		INSERT INTO tracks (directory_id, path, title, artist, album, track_number, year, genre, duration_seconds, artist_id, album_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, t.DirectoryID, t.Path, t.Title, t.Artist, t.Album, t.TrackNumber, t.Year, t.Genre, t.DurationSeconds, artistID, albumID); err != nil {
		return fmt.Errorf("inserting track: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing track insert: %w", err)
	}

	// Saving embedded artwork happens after commit, outside the
	// transaction — it's disk I/O (resizing), not something that needs
	// the track/artist/album row's atomicity. AC-1: only the first
	// embedded image found for this album wins, so the UPDATE is
	// conditioned on the album still being art_status = 'pending' — a
	// second track's artwork arriving after the first one already landed
	// (including a concurrent scan of a second directory covering the
	// same album) is a no-op, not an overwrite. A save/record failure
	// here is logged, not returned — the track itself imported fine.
	if s.images != nil && len(t.ArtworkData) > 0 {
		s.saveEmbeddedAlbumArt(ctx, albumID, t.ArtworkData)
	}
	return nil
}

func (s *Store) saveEmbeddedAlbumArt(ctx context.Context, albumID int64, data []byte) {
	thumbURL, fullURL, err := s.images.Save("album", albumID, data)
	if err != nil {
		log.Printf("library: album %d: saving embedded artwork: %v", albumID, err)
		return
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE albums SET art_status = 'found', cover_thumb_path = $2, cover_path = $3
		WHERE id = $1 AND art_status = 'pending'
	`, albumID, thumbURL, fullURL); err != nil {
		log.Printf("library: album %d: recording embedded artwork: %v", albumID, err)
	}
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
