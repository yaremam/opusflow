package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// FindArtistIDByMusicBrainzID returns the ID of an artist (other than
// excludeID) already carrying mbid, if one exists — used by the
// background enrichment job (TDR 017) to notice two rows have resolved to
// the same real-world artist. A blank mbid never matches anything.
func (s *Store) FindArtistIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error) {
	if mbid == "" {
		return 0, false, nil
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM artists WHERE musicbrainz_id = $1 AND id != $2 LIMIT 1
	`, mbid, excludeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("finding artist by musicbrainz id: %w", err)
	}
	return id, true, nil
}

// FindAlbumIDByMusicBrainzID is FindArtistIDByMusicBrainzID's album
// counterpart, matching on release-group MBID.
func (s *Store) FindAlbumIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error) {
	if mbid == "" {
		return 0, false, nil
	}
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM albums WHERE musicbrainz_id = $1 AND id != $2 LIMIT 1
	`, mbid, excludeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("finding album by musicbrainz id: %w", err)
	}
	return id, true, nil
}

// MergeArtists reassigns every album, track, and gallery photo from
// loserID onto winnerID, then removes the now-empty loserID row. Used by
// the background enrichment job when two artist rows turn out to share a
// MusicBrainz ID (TDR 017), and by the manual "Merge into..." action
// (TDR 018). Files already copied to disk are left exactly where they
// are — only the catalog association moves, matching how no feature in
// this app renames on-disk files after import today.
func (s *Store) MergeArtists(ctx context.Context, loserID, winnerID int64) error {
	if loserID == winnerID {
		return fmt.Errorf("cannot merge artist %d into itself", loserID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id, title FROM albums WHERE artist_id = $1`, loserID)
	if err != nil {
		return fmt.Errorf("listing loser's albums: %w", err)
	}
	type albumRow struct {
		id    int64
		title string
	}
	var loserAlbums []albumRow
	for rows.Next() {
		var a albumRow
		if err := rows.Scan(&a.id, &a.title); err != nil {
			rows.Close()
			return fmt.Errorf("scanning loser album: %w", err)
		}
		loserAlbums = append(loserAlbums, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, la := range loserAlbums {
		var winnerAlbumID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM albums WHERE artist_id = $1 AND title = $2`, winnerID, la.title).Scan(&winnerAlbumID)
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := tx.ExecContext(ctx, `UPDATE albums SET artist_id = $1 WHERE id = $2`, winnerID, la.id); err != nil {
				return fmt.Errorf("reassigning album %d: %w", la.id, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("checking for a same-titled album: %w", err)
		}
		if err := mergeAlbumRows(ctx, tx, la.id, winnerAlbumID); err != nil {
			return fmt.Errorf("folding same-titled album %d into %d: %w", la.id, winnerAlbumID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE tracks SET artist_id = $1 WHERE artist_id = $2`, winnerID, loserID); err != nil {
		return fmt.Errorf("reassigning tracks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE artist_photos SET artist_id = $1 WHERE artist_id = $2`, winnerID, loserID); err != nil {
		return fmt.Errorf("reassigning artist photos: %w", err)
	}
	if err := dedupeSingleFlag(ctx, tx, "artist_photos", "artist_id", winnerID, "is_primary"); err != nil {
		return fmt.Errorf("deduping primary photo: %w", err)
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM artists WHERE id = $1`, loserID)
	if err != nil {
		return fmt.Errorf("deleting merged artist: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrArtistNotFound
	}

	return tx.Commit()
}

// MergeAlbums reassigns every track and gallery cover from loserID onto
// winnerID, then removes the now-empty loserID row. Only defined for two
// albums under the same artist — merging across artists isn't an album
// merge, it's an artist merge (MergeArtists). Used by the background
// enrichment job (TDR 017, including internally by MergeArtists for a
// same-titled-album collision) and the manual merge tool (TDR 018).
func (s *Store) MergeAlbums(ctx context.Context, loserID, winnerID int64) error {
	if loserID == winnerID {
		return fmt.Errorf("cannot merge album %d into itself", loserID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var loserArtistID, winnerArtistID int64
	err = tx.QueryRowContext(ctx, `SELECT artist_id FROM albums WHERE id = $1`, loserID).Scan(&loserArtistID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAlbumNotFound
	}
	if err != nil {
		return fmt.Errorf("looking up loser album's artist: %w", err)
	}
	err = tx.QueryRowContext(ctx, `SELECT artist_id FROM albums WHERE id = $1`, winnerID).Scan(&winnerArtistID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrAlbumNotFound
	}
	if err != nil {
		return fmt.Errorf("looking up winner album's artist: %w", err)
	}
	if loserArtistID != winnerArtistID {
		return fmt.Errorf("cannot merge album %d into %d: they belong to different artists", loserID, winnerID)
	}

	if err := mergeAlbumRows(ctx, tx, loserID, winnerID); err != nil {
		return err
	}
	return tx.Commit()
}

// mergeAlbumRows reassigns loserAlbumID's tracks and covers onto
// winnerAlbumID, dedupes the merged cover gallery's primary flag, then
// deletes the now-empty loser album row. Shared by MergeAlbums and
// MergeArtists's same-titled-album fold — callers own their own
// transaction and, for MergeArtists's case, the artist_id reassignment
// (this only ever touches album_id, never artist_id).
func mergeAlbumRows(ctx context.Context, tx *sql.Tx, loserAlbumID, winnerAlbumID int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE tracks SET album_id = $1 WHERE album_id = $2`, winnerAlbumID, loserAlbumID); err != nil {
		return fmt.Errorf("reassigning tracks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE album_covers SET album_id = $1 WHERE album_id = $2`, winnerAlbumID, loserAlbumID); err != nil {
		return fmt.Errorf("reassigning album covers: %w", err)
	}
	if err := dedupeSingleFlag(ctx, tx, "album_covers", "album_id", winnerAlbumID, "is_primary"); err != nil {
		return fmt.Errorf("deduping primary cover: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM albums WHERE id = $1`, loserAlbumID); err != nil {
		return fmt.Errorf("deleting merged album: %w", err)
	}
	return nil
}

// dedupeSingleFlag ensures at most one row in table (scoped by
// fkCol = fkID) has flagCol = true, keeping whichever already was (oldest
// on a tie) and clearing the rest. Needed after a merge: two
// independently-primary galleries colliding into one row's gallery would
// otherwise leave two rows flagged true at once, breaking the "exactly
// one primary" invariant TDR 014 already relies on elsewhere
// (setGalleryPrimary, deleteGalleryRow's promotion). Parameterized by
// flagCol (not hardcoded to "is_primary") so covering a future second
// flag, like TDR 016's "banner", is a one-line addition at the call site.
func dedupeSingleFlag(ctx context.Context, tx *sql.Tx, table, fkCol string, fkID int64, flagCol string) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET %s = FALSE WHERE %s = $1 AND %s = TRUE AND id != (
			SELECT id FROM %s WHERE %s = $1 AND %s = TRUE ORDER BY created_at ASC, id ASC LIMIT 1
		)
	`, table, flagCol, fkCol, flagCol, table, fkCol, flagCol), fkID)
	if err != nil {
		return fmt.Errorf("deduping %s.%s: %w", table, flagCol, err)
	}
	return nil
}
