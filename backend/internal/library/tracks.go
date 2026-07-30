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

	var fileSizeBytes any
	if t.FileSizeBytes > 0 {
		fileSizeBytes = t.FileSizeBytes
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tracks (import_id, path, title, artist, album, track_number, year, genre, duration_seconds, file_size_bytes, artist_id, album_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, t.ImportID, t.Path, t.Title, t.Artist, t.Album, t.TrackNumber, t.Year, t.Genre, t.DurationSeconds, fileSizeBytes, artistID, albumID); err != nil {
		return fmt.Errorf("inserting track: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing track insert: %w", err)
	}

	// Saving embedded artwork happens after commit, outside the
	// transaction — it's disk I/O (resizing), not something that needs
	// the track/artist/album row's atomicity. AC-7: every embedded
	// picture is added to the album's gallery, not just the first one
	// found — content-hash dedup (already handled by AddAlbumCover)
	// naturally collapses the common case of every track on an album
	// sharing the same embedded cover down to one gallery entry, without
	// this needing its own gating. A save/add failure here is logged, not
	// returned — the track itself imported fine.
	if s.images != nil && len(t.ArtworkPictures) > 0 {
		s.saveEmbeddedAlbumArt(ctx, albumID, t.ArtworkPictures)
	}
	return nil
}

func (s *Store) saveEmbeddedAlbumArt(ctx context.Context, albumID int64, pictures []organize.EmbeddedPicture) {
	for _, pic := range pictures {
		thumbURL, fullURL, hash, err := s.images.Save("album", albumID, pic.Data)
		if err != nil {
			log.Printf("library: album %d: saving embedded artwork: %v", albumID, err)
			continue
		}
		if _, err := s.AddAlbumCover(ctx, albumID, thumbURL, fullURL, "embedded", pic.PictureType, hash); err != nil {
			log.Printf("library: album %d: adding embedded artwork: %v", albumID, err)
		}
	}
	// Mark this album's art settled now that local embedded art exists —
	// a no-op if some earlier track/lookup already did (SetAlbumArtIfOpen
	// only writes while still pending/failed), so this is safe to call
	// once per track without re-triggering anything.
	if err := s.SetAlbumArtIfOpen(ctx, albumID, enrich.Found, "", ""); err != nil {
		log.Printf("library: album %d: recording embedded artwork status: %v", albumID, err)
	}
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
