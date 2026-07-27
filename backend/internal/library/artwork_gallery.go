package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrArtistPhotoNotFound is returned when a photo ID doesn't match any
// photo in the given artist's gallery.
var ErrArtistPhotoNotFound = errors.New("artist photo not found")

// ErrAlbumCoverNotFound is returned when a cover ID doesn't match any
// cover in the given album's gallery.
var ErrAlbumCoverNotFound = errors.New("album cover not found")

// ArtistPhoto is one image in an artist's photo gallery (TDR 014).
// Exactly one photo per artist has IsPrimary set — that's the one used as
// the artist's thumbnail in list views and grid tiles.
type ArtistPhoto struct {
	ID        int64     `json:"id"`
	ThumbURL  string    `json:"thumbUrl"`
	FullURL   string    `json:"fullUrl"`
	Source    string    `json:"source"`
	IsPrimary bool      `json:"isPrimary"`
	CreatedAt time.Time `json:"createdAt"`
}

// AlbumCover is one image in an album's cover gallery (TDR 014).
// PictureType is the Cover Art Archive/APIC/PICTURE type label ("front",
// "back", "booklet", ...) when known, empty for manual uploads.
type AlbumCover struct {
	ID          int64     `json:"id"`
	ThumbURL    string    `json:"thumbUrl"`
	FullURL     string    `json:"fullUrl"`
	Source      string    `json:"source"`
	PictureType string    `json:"pictureType,omitempty"`
	IsPrimary   bool      `json:"isPrimary"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListArtistPhotos returns every photo in an artist's gallery, in the
// order they were added.
func (s *Store) ListArtistPhotos(ctx context.Context, artistID int64) ([]ArtistPhoto, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, thumb_path, full_path, source, is_primary, created_at
		FROM artist_photos WHERE artist_id = $1
		ORDER BY created_at ASC, id ASC
	`, artistID)
	if err != nil {
		return nil, fmt.Errorf("listing artist photos: %w", err)
	}
	defer rows.Close()

	photos := []ArtistPhoto{}
	for rows.Next() {
		var p ArtistPhoto
		if err := rows.Scan(&p.ID, &p.ThumbURL, &p.FullURL, &p.Source, &p.IsPrimary, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning artist photo: %w", err)
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return photos, nil
}

// AddArtistPhoto adds a new photo to an artist's gallery, becoming
// primary if it's the first (AC-2). If contentHash matches a photo
// already in the gallery, the existing photo is returned instead of
// inserting a duplicate (AC-5) — a blank contentHash (legacy rows with no
// known hash) never dedupes against anything.
func (s *Store) AddArtistPhoto(ctx context.Context, artistID int64, thumbURL, fullURL, source, contentHash string) (ArtistPhoto, error) {
	var p ArtistPhoto
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO artist_photos (artist_id, thumb_path, full_path, source, content_hash, is_primary)
		SELECT $1, $2, $3, $4, $5, NOT EXISTS (SELECT 1 FROM artist_photos WHERE artist_id = $1)
		WHERE $5 = '' OR NOT EXISTS (SELECT 1 FROM artist_photos WHERE artist_id = $1 AND content_hash = $5)
		RETURNING id, thumb_path, full_path, source, is_primary, created_at
	`, artistID, thumbURL, fullURL, source, contentHash).Scan(&p.ID, &p.ThumbURL, &p.FullURL, &p.Source, &p.IsPrimary, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, thumb_path, full_path, source, is_primary, created_at
			FROM artist_photos WHERE artist_id = $1 AND content_hash = $2
		`, artistID, contentHash).Scan(&p.ID, &p.ThumbURL, &p.FullURL, &p.Source, &p.IsPrimary, &p.CreatedAt)
	}
	if err != nil {
		return ArtistPhoto{}, fmt.Errorf("adding artist photo: %w", err)
	}
	return p, nil
}

// SetArtistPrimaryPhoto marks photoID as the artist's primary photo,
// clearing the flag from every other photo in the gallery.
func (s *Store) SetArtistPrimaryPhoto(ctx context.Context, artistID, photoID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE artist_photos SET is_primary = FALSE WHERE artist_id = $1`, artistID); err != nil {
		return fmt.Errorf("clearing existing primary photo: %w", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE artist_photos SET is_primary = TRUE WHERE id = $1 AND artist_id = $2`, photoID, artistID)
	if err != nil {
		return fmt.Errorf("setting primary photo: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrArtistPhotoNotFound
	}
	return tx.Commit()
}

// DeleteArtistPhoto removes a photo from an artist's gallery, promoting
// the oldest remaining photo to primary if the deleted photo was primary
// (AC-2 requires exactly one primary photo whenever the gallery is
// non-empty). Returns the deleted photo's own thumb/full paths so the
// caller can decide whether to also remove the underlying files (AC-4).
func (s *Store) DeleteArtistPhoto(ctx context.Context, artistID, photoID int64) (thumbPath, fullPath string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var wasPrimary bool
	err = tx.QueryRowContext(ctx, `
		DELETE FROM artist_photos WHERE id = $1 AND artist_id = $2
		RETURNING thumb_path, full_path, is_primary
	`, photoID, artistID).Scan(&thumbPath, &fullPath, &wasPrimary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrArtistPhotoNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("deleting artist photo: %w", err)
	}

	if wasPrimary {
		if _, err := tx.ExecContext(ctx, `
			UPDATE artist_photos SET is_primary = TRUE WHERE id = (
				SELECT id FROM artist_photos WHERE artist_id = $1
				ORDER BY created_at ASC, id ASC LIMIT 1
			)
		`, artistID); err != nil {
			return "", "", fmt.Errorf("promoting new primary photo: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("committing: %w", err)
	}
	return thumbPath, fullPath, nil
}

// ListAlbumCovers returns every cover in an album's gallery, in the order
// they were added.
func (s *Store) ListAlbumCovers(ctx context.Context, albumID int64) ([]AlbumCover, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, thumb_path, full_path, source, picture_type, is_primary, created_at
		FROM album_covers WHERE album_id = $1
		ORDER BY created_at ASC, id ASC
	`, albumID)
	if err != nil {
		return nil, fmt.Errorf("listing album covers: %w", err)
	}
	defer rows.Close()

	covers := []AlbumCover{}
	for rows.Next() {
		var c AlbumCover
		if err := rows.Scan(&c.ID, &c.ThumbURL, &c.FullURL, &c.Source, &c.PictureType, &c.IsPrimary, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning album cover: %w", err)
		}
		covers = append(covers, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return covers, nil
}

// AddAlbumCover adds a new cover to an album's gallery, becoming primary
// if it's the first (AC-2). If contentHash matches a cover already in the
// gallery, the existing cover is returned instead of inserting a
// duplicate (AC-5) — a blank contentHash never dedupes against anything.
func (s *Store) AddAlbumCover(ctx context.Context, albumID int64, thumbURL, fullURL, source, pictureType, contentHash string) (AlbumCover, error) {
	var c AlbumCover
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO album_covers (album_id, thumb_path, full_path, source, picture_type, content_hash, is_primary)
		SELECT $1, $2, $3, $4, $5, $6, NOT EXISTS (SELECT 1 FROM album_covers WHERE album_id = $1)
		WHERE $6 = '' OR NOT EXISTS (SELECT 1 FROM album_covers WHERE album_id = $1 AND content_hash = $6)
		RETURNING id, thumb_path, full_path, source, picture_type, is_primary, created_at
	`, albumID, thumbURL, fullURL, source, pictureType, contentHash).Scan(&c.ID, &c.ThumbURL, &c.FullURL, &c.Source, &c.PictureType, &c.IsPrimary, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, thumb_path, full_path, source, picture_type, is_primary, created_at
			FROM album_covers WHERE album_id = $1 AND content_hash = $2
		`, albumID, contentHash).Scan(&c.ID, &c.ThumbURL, &c.FullURL, &c.Source, &c.PictureType, &c.IsPrimary, &c.CreatedAt)
	}
	if err != nil {
		return AlbumCover{}, fmt.Errorf("adding album cover: %w", err)
	}
	return c, nil
}

// SetAlbumPrimaryCover marks coverID as the album's primary cover,
// clearing the flag from every other cover in the gallery.
func (s *Store) SetAlbumPrimaryCover(ctx context.Context, albumID, coverID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE album_covers SET is_primary = FALSE WHERE album_id = $1`, albumID); err != nil {
		return fmt.Errorf("clearing existing primary cover: %w", err)
	}
	res, err := tx.ExecContext(ctx, `UPDATE album_covers SET is_primary = TRUE WHERE id = $1 AND album_id = $2`, coverID, albumID)
	if err != nil {
		return fmt.Errorf("setting primary cover: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAlbumCoverNotFound
	}
	return tx.Commit()
}

// DeleteAlbumCover removes a cover from an album's gallery, promoting the
// oldest remaining cover to primary if the deleted cover was primary.
// Returns the deleted cover's own thumb/full paths so the caller can
// decide whether to also remove the underlying files (AC-4).
func (s *Store) DeleteAlbumCover(ctx context.Context, albumID, coverID int64) (thumbPath, fullPath string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var wasPrimary bool
	err = tx.QueryRowContext(ctx, `
		DELETE FROM album_covers WHERE id = $1 AND album_id = $2
		RETURNING thumb_path, full_path, is_primary
	`, coverID, albumID).Scan(&thumbPath, &fullPath, &wasPrimary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrAlbumCoverNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("deleting album cover: %w", err)
	}

	if wasPrimary {
		if _, err := tx.ExecContext(ctx, `
			UPDATE album_covers SET is_primary = TRUE WHERE id = (
				SELECT id FROM album_covers WHERE album_id = $1
				ORDER BY created_at ASC, id ASC LIMIT 1
			)
		`, albumID); err != nil {
			return "", "", fmt.Errorf("promoting new primary cover: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("committing: %w", err)
	}
	return thumbPath, fullPath, nil
}
