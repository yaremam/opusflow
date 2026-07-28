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
// the artist's thumbnail in list views and grid tiles. Independently, at
// most one photo has IsBanner set — that's the one used as the detail
// page's header banner (TDR 016); unlike IsPrimary it may be unset for a
// gallery that hasn't had a banner chosen yet.
type ArtistPhoto struct {
	ID        int64     `json:"id"`
	ThumbURL  string    `json:"thumbUrl"`
	FullURL   string    `json:"fullUrl"`
	Source    string    `json:"source"`
	IsPrimary bool      `json:"isPrimary"`
	IsBanner  bool      `json:"isBanner"`
	CreatedAt time.Time `json:"createdAt"`
}

// AlbumCover is one image in an album's cover gallery (TDR 014).
// PictureType is the Cover Art Archive/APIC/PICTURE type label ("front",
// "back", "booklet", ...) when known, empty for manual uploads. IsBanner
// is TDR 016's independent, optional "used as the header banner" flag —
// see ArtistPhoto.IsBanner.
type AlbumCover struct {
	ID          int64     `json:"id"`
	ThumbURL    string    `json:"thumbUrl"`
	FullURL     string    `json:"fullUrl"`
	Source      string    `json:"source"`
	PictureType string    `json:"pictureType,omitempty"`
	IsPrimary   bool      `json:"isPrimary"`
	IsBanner    bool      `json:"isBanner"`
	CreatedAt   time.Time `json:"createdAt"`
}

// galleryRow is the one shape both artist_photos and album_covers rows
// scan into internally — PictureType is simply always "" for a table that
// has no such column. ArtistPhoto/AlbumCover stay separate public types
// (callers and JSON responses depend on their exact shape); this is only
// the implementation's shared currency between the two.
type galleryRow struct {
	ID          int64
	ThumbURL    string
	FullURL     string
	Source      string
	PictureType string
	IsPrimary   bool
	IsBanner    bool
	CreatedAt   time.Time
}

// galleryTable names the physical table and FK column backing one gallery
// — the only thing that varies between an artist's photos and an album's
// covers. hasPictureType gates the one column album_covers has that
// artist_photos doesn't.
type galleryTable struct {
	name           string
	fkColumn       string
	hasPictureType bool
}

var artistPhotoTable = galleryTable{name: "artist_photos", fkColumn: "artist_id"}
var albumCoverTable = galleryTable{name: "album_covers", fkColumn: "album_id", hasPictureType: true}

// listGalleryRows returns every row in one entity's gallery, in the order
// they were added — the shared implementation behind
// ListArtistPhotos/ListAlbumCovers.
func (s *Store) listGalleryRows(ctx context.Context, t galleryTable, entityID int64) ([]galleryRow, error) {
	cols := "id, thumb_path, full_path, source, is_primary, is_banner, created_at"
	if t.hasPictureType {
		cols = "id, thumb_path, full_path, source, picture_type, is_primary, is_banner, created_at"
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT %s FROM %s WHERE %s = $1
		ORDER BY created_at ASC, id ASC
	`, cols, t.name, t.fkColumn), entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := []galleryRow{}
	for rows.Next() {
		var g galleryRow
		var scanErr error
		if t.hasPictureType {
			scanErr = rows.Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.PictureType, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
		} else {
			scanErr = rows.Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
		}
		if scanErr != nil {
			return nil, scanErr
		}
		images = append(images, g)
	}
	return images, rows.Err()
}

// insertGalleryRow adds a new image to one entity's gallery, becoming
// primary if it's the first (AC-2). If contentHash matches an image
// already in the gallery, the existing row is returned instead of
// inserting a duplicate (AC-5) — a blank contentHash (legacy rows with no
// known hash) never dedupes against anything. Shared implementation
// behind AddArtistPhoto/AddAlbumCover.
func (s *Store) insertGalleryRow(ctx context.Context, t galleryTable, entityID int64, thumbURL, fullURL, source, pictureType, contentHash string) (galleryRow, error) {
	var g galleryRow
	var err error
	if t.hasPictureType {
		err = s.db.QueryRowContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (%s, thumb_path, full_path, source, picture_type, content_hash, is_primary)
			SELECT $1, $2, $3, $4, $5, $6, NOT EXISTS (SELECT 1 FROM %s WHERE %s = $1)
			WHERE $6 = '' OR NOT EXISTS (SELECT 1 FROM %s WHERE %s = $1 AND content_hash = $6)
			RETURNING id, thumb_path, full_path, source, picture_type, is_primary, is_banner, created_at
		`, t.name, t.fkColumn, t.name, t.fkColumn, t.name, t.fkColumn),
			entityID, thumbURL, fullURL, source, pictureType, contentHash,
		).Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.PictureType, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
	} else {
		err = s.db.QueryRowContext(ctx, fmt.Sprintf(`
			INSERT INTO %s (%s, thumb_path, full_path, source, content_hash, is_primary)
			SELECT $1, $2, $3, $4, $5, NOT EXISTS (SELECT 1 FROM %s WHERE %s = $1)
			WHERE $5 = '' OR NOT EXISTS (SELECT 1 FROM %s WHERE %s = $1 AND content_hash = $5)
			RETURNING id, thumb_path, full_path, source, is_primary, is_banner, created_at
		`, t.name, t.fkColumn, t.name, t.fkColumn, t.name, t.fkColumn),
			entityID, thumbURL, fullURL, source, contentHash,
		).Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		selectCols := "id, thumb_path, full_path, source, is_primary, is_banner, created_at"
		if t.hasPictureType {
			selectCols = "id, thumb_path, full_path, source, picture_type, is_primary, is_banner, created_at"
		}
		row := s.db.QueryRowContext(ctx, fmt.Sprintf(`
			SELECT %s FROM %s WHERE %s = $1 AND content_hash = $2
		`, selectCols, t.name, t.fkColumn), entityID, contentHash)
		if t.hasPictureType {
			err = row.Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.PictureType, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
		} else {
			err = row.Scan(&g.ID, &g.ThumbURL, &g.FullURL, &g.Source, &g.IsPrimary, &g.IsBanner, &g.CreatedAt)
		}
	}
	if err != nil {
		return galleryRow{}, err
	}
	return g, nil
}

// setGalleryFlag marks imageID as t's column-flagged image for entityID
// (column is "is_primary" or "is_banner"), clearing the flag from every
// other image in the same gallery first — shared implementation behind
// SetArtistPrimaryPhoto/SetAlbumPrimaryCover and
// SetArtistBannerPhoto/SetAlbumBannerCover (TDR 016). Both flags are
// independent columns on the same row, so setting one never touches the
// other — an image is free to be both primary and banner at once.
func (s *Store) setGalleryFlag(ctx context.Context, t galleryTable, entityID, imageID int64, column string, notFound error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET %s = FALSE WHERE %s = $1`, t.name, column, t.fkColumn), entityID); err != nil {
		return fmt.Errorf("clearing existing %s: %w", column, err)
	}
	res, err := tx.ExecContext(ctx, fmt.Sprintf(`UPDATE %s SET %s = TRUE WHERE id = $1 AND %s = $2`, t.name, column, t.fkColumn), imageID, entityID)
	if err != nil {
		return fmt.Errorf("setting %s: %w", column, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound
	}
	return tx.Commit()
}

// deleteGalleryRow removes an image from one entity's gallery, promoting
// the oldest remaining image to primary if the deleted one was primary
// (AC-2 requires exactly one primary image whenever the gallery is
// non-empty). Returns the deleted image's own thumb/full paths so the
// caller can decide whether to also remove the underlying files (AC-4).
// Shared implementation behind DeleteArtistPhoto/DeleteAlbumCover.
func (s *Store) deleteGalleryRow(ctx context.Context, t galleryTable, entityID, imageID int64, notFound error) (thumbPath, fullPath string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var wasPrimary bool
	err = tx.QueryRowContext(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE id = $1 AND %s = $2
		RETURNING thumb_path, full_path, is_primary
	`, t.name, t.fkColumn), imageID, entityID).Scan(&thumbPath, &fullPath, &wasPrimary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", notFound
	}
	if err != nil {
		return "", "", fmt.Errorf("deleting: %w", err)
	}

	if wasPrimary {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET is_primary = TRUE WHERE id = (
				SELECT id FROM %s WHERE %s = $1
				ORDER BY created_at ASC, id ASC LIMIT 1
			)
		`, t.name, t.name, t.fkColumn), entityID); err != nil {
			return "", "", fmt.Errorf("promoting new primary: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", "", fmt.Errorf("committing: %w", err)
	}
	return thumbPath, fullPath, nil
}

// ListArtistPhotos returns every photo in an artist's gallery, in the
// order they were added.
func (s *Store) ListArtistPhotos(ctx context.Context, artistID int64) ([]ArtistPhoto, error) {
	rows, err := s.listGalleryRows(ctx, artistPhotoTable, artistID)
	if err != nil {
		return nil, fmt.Errorf("listing artist photos: %w", err)
	}
	photos := make([]ArtistPhoto, len(rows))
	for i, r := range rows {
		photos[i] = ArtistPhoto{ID: r.ID, ThumbURL: r.ThumbURL, FullURL: r.FullURL, Source: r.Source, IsPrimary: r.IsPrimary, IsBanner: r.IsBanner, CreatedAt: r.CreatedAt}
	}
	return photos, nil
}

// AddArtistPhoto adds a new photo to an artist's gallery, becoming
// primary if it's the first (AC-2). If contentHash matches a photo
// already in the gallery, the existing photo is returned instead of
// inserting a duplicate (AC-5) — a blank contentHash (legacy rows with no
// known hash) never dedupes against anything.
func (s *Store) AddArtistPhoto(ctx context.Context, artistID int64, thumbURL, fullURL, source, contentHash string) (ArtistPhoto, error) {
	r, err := s.insertGalleryRow(ctx, artistPhotoTable, artistID, thumbURL, fullURL, source, "", contentHash)
	if err != nil {
		return ArtistPhoto{}, fmt.Errorf("adding artist photo: %w", err)
	}
	return ArtistPhoto{ID: r.ID, ThumbURL: r.ThumbURL, FullURL: r.FullURL, Source: r.Source, IsPrimary: r.IsPrimary, IsBanner: r.IsBanner, CreatedAt: r.CreatedAt}, nil
}

// SetArtistPrimaryPhoto marks photoID as the artist's primary photo,
// clearing the flag from every other photo in the gallery.
func (s *Store) SetArtistPrimaryPhoto(ctx context.Context, artistID, photoID int64) error {
	return s.setGalleryFlag(ctx, artistPhotoTable, artistID, photoID, "is_primary", ErrArtistPhotoNotFound)
}

// SetArtistBannerPhoto marks photoID as the artist's header-banner photo
// (TDR 016), clearing the flag from every other photo in the gallery.
// Independent of IsPrimary — see ArtistPhoto.IsBanner.
func (s *Store) SetArtistBannerPhoto(ctx context.Context, artistID, photoID int64) error {
	return s.setGalleryFlag(ctx, artistPhotoTable, artistID, photoID, "is_banner", ErrArtistPhotoNotFound)
}

// DeleteArtistPhoto removes a photo from an artist's gallery, promoting
// the oldest remaining photo to primary if the deleted photo was primary
// (AC-2 requires exactly one primary photo whenever the gallery is
// non-empty). Returns the deleted photo's own thumb/full paths so the
// caller can decide whether to also remove the underlying files (AC-4).
func (s *Store) DeleteArtistPhoto(ctx context.Context, artistID, photoID int64) (thumbPath, fullPath string, err error) {
	return s.deleteGalleryRow(ctx, artistPhotoTable, artistID, photoID, ErrArtistPhotoNotFound)
}

// ListAlbumCovers returns every cover in an album's gallery, in the order
// they were added.
func (s *Store) ListAlbumCovers(ctx context.Context, albumID int64) ([]AlbumCover, error) {
	rows, err := s.listGalleryRows(ctx, albumCoverTable, albumID)
	if err != nil {
		return nil, fmt.Errorf("listing album covers: %w", err)
	}
	covers := make([]AlbumCover, len(rows))
	for i, r := range rows {
		covers[i] = AlbumCover{ID: r.ID, ThumbURL: r.ThumbURL, FullURL: r.FullURL, Source: r.Source, PictureType: r.PictureType, IsPrimary: r.IsPrimary, IsBanner: r.IsBanner, CreatedAt: r.CreatedAt}
	}
	return covers, nil
}

// AddAlbumCover adds a new cover to an album's gallery, becoming primary
// if it's the first (AC-2). If contentHash matches a cover already in the
// gallery, the existing cover is returned instead of inserting a
// duplicate (AC-5) — a blank contentHash never dedupes against anything.
func (s *Store) AddAlbumCover(ctx context.Context, albumID int64, thumbURL, fullURL, source, pictureType, contentHash string) (AlbumCover, error) {
	r, err := s.insertGalleryRow(ctx, albumCoverTable, albumID, thumbURL, fullURL, source, pictureType, contentHash)
	if err != nil {
		return AlbumCover{}, fmt.Errorf("adding album cover: %w", err)
	}
	return AlbumCover{ID: r.ID, ThumbURL: r.ThumbURL, FullURL: r.FullURL, Source: r.Source, PictureType: r.PictureType, IsPrimary: r.IsPrimary, IsBanner: r.IsBanner, CreatedAt: r.CreatedAt}, nil
}

// AddAlbumCoverForEnrichment is AddAlbumCover, discarding the created row —
// the shape enrich.Store's Store interface needs. enrich can't reference
// this package's AlbumCover type without an import cycle (library already
// imports enrich for enrich.Status), and Job never needs the created row
// back, only whether the add succeeded.
func (s *Store) AddAlbumCoverForEnrichment(ctx context.Context, albumID int64, thumbURL, fullURL, source, pictureType, contentHash string) error {
	_, err := s.AddAlbumCover(ctx, albumID, thumbURL, fullURL, source, pictureType, contentHash)
	return err
}

// SetAlbumPrimaryCover marks coverID as the album's primary cover,
// clearing the flag from every other cover in the gallery.
func (s *Store) SetAlbumPrimaryCover(ctx context.Context, albumID, coverID int64) error {
	return s.setGalleryFlag(ctx, albumCoverTable, albumID, coverID, "is_primary", ErrAlbumCoverNotFound)
}

// SetAlbumBannerCover marks coverID as the album's header-banner cover
// (TDR 016), clearing the flag from every other cover in the gallery.
// Independent of IsPrimary — see AlbumCover.IsBanner.
func (s *Store) SetAlbumBannerCover(ctx context.Context, albumID, coverID int64) error {
	return s.setGalleryFlag(ctx, albumCoverTable, albumID, coverID, "is_banner", ErrAlbumCoverNotFound)
}

// DeleteAlbumCover removes a cover from an album's gallery, promoting the
// oldest remaining cover to primary if the deleted cover was primary.
// Returns the deleted cover's own thumb/full paths so the caller can
// decide whether to also remove the underlying files (AC-4).
func (s *Store) DeleteAlbumCover(ctx context.Context, albumID, coverID int64) (thumbPath, fullPath string, err error) {
	return s.deleteGalleryRow(ctx, albumCoverTable, albumID, coverID, ErrAlbumCoverNotFound)
}
