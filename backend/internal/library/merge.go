package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// findIDByMusicBrainzID is FindArtistIDByMusicBrainzID/
// FindAlbumIDByMusicBrainzID's shared implementation, parameterized by
// table the same way galleryTable-based helpers elsewhere in this
// package are. A blank mbid never matches anything.
func findIDByMusicBrainzID(ctx context.Context, db *sql.DB, table, mbid string, excludeID int64) (int64, bool, error) {
	if mbid == "" {
		return 0, false, nil
	}
	var id int64
	err := db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE musicbrainz_id = $1 AND id != $2 LIMIT 1
	`, table), mbid, excludeID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("finding %s by musicbrainz id: %w", table, err)
	}
	return id, true, nil
}

// FindArtistIDByMusicBrainzID returns the ID of an artist (other than
// excludeID) already carrying mbid, if one exists — used by the
// background enrichment job (TDR 017) to notice two rows have resolved to
// the same real-world artist.
func (s *Store) FindArtistIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error) {
	return findIDByMusicBrainzID(ctx, s.db, "artists", mbid, excludeID)
}

// FindAlbumIDByMusicBrainzID is FindArtistIDByMusicBrainzID's album
// counterpart, matching on release-group MBID.
func (s *Store) FindAlbumIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error) {
	return findIDByMusicBrainzID(ctx, s.db, "albums", mbid, excludeID)
}

// ErrCannotMergeIntoSelf is returned by MergeArtists/MergeAlbums when
// loserID and winnerID are the same row — always a caller bug (TDR 017's
// automatic dedup never constructs this pair) or bad client input (TDR
// 018's manual merge tool), never a legitimate merge.
var ErrCannotMergeIntoSelf = errors.New("cannot merge an entity into itself")

// ErrAlbumsBelongToDifferentArtists is returned by MergeAlbums when the
// two albums aren't under the same artist — that's an artist merge, not
// an album one (see MergeAlbums's doc comment).
var ErrAlbumsBelongToDifferentArtists = errors.New("albums belong to different artists")

// MergeArtists reassigns every album, track, and gallery photo from
// loserID onto winnerID, then removes the now-empty loserID row. Used by
// the background enrichment job when two artist rows turn out to share a
// MusicBrainz ID (TDR 017), and by the manual "Merge into..." action
// (TDR 018). Files already copied to disk are left exactly where they
// are — only the catalog association moves, matching how no feature in
// this app renames on-disk files after import today.
func (s *Store) MergeArtists(ctx context.Context, loserID, winnerID int64) error {
	if loserID == winnerID {
		return ErrCannotMergeIntoSelf
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	loserAlbums, err := queryAlbumTitles(ctx, tx, loserID)
	if err != nil {
		return fmt.Errorf("listing loser's albums: %w", err)
	}
	winnerAlbumsByTitle, err := queryAlbumTitles(ctx, tx, winnerID)
	if err != nil {
		return fmt.Errorf("listing winner's albums: %w", err)
	}
	winnerAlbumIDByTitle := make(map[string]int64, len(winnerAlbumsByTitle))
	for _, wa := range winnerAlbumsByTitle {
		winnerAlbumIDByTitle[wa.title] = wa.id
	}

	for _, la := range loserAlbums {
		if winnerAlbumID, collides := winnerAlbumIDByTitle[la.title]; collides {
			if err := mergeAlbumRows(ctx, tx, la.id, winnerAlbumID); err != nil {
				return fmt.Errorf("folding same-titled album %d into %d: %w", la.id, winnerAlbumID, err)
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE albums SET artist_id = $1 WHERE id = $2`, winnerID, la.id); err != nil {
			return fmt.Errorf("reassigning album %d: %w", la.id, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE tracks SET artist_id = $1 WHERE artist_id = $2`, winnerID, loserID); err != nil {
		return fmt.Errorf("reassigning tracks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE artist_photos SET artist_id = $1 WHERE artist_id = $2`, winnerID, loserID); err != nil {
		return fmt.Errorf("reassigning artist photos: %w", err)
	}
	if err := dedupeSingleFlag(ctx, tx, artistPhotoTable, winnerID); err != nil {
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
		return ErrCannotMergeIntoSelf
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id, artist_id FROM albums WHERE id IN ($1, $2)`, loserID, winnerID)
	if err != nil {
		return fmt.Errorf("looking up albums' artists: %w", err)
	}
	artistIDByAlbum := make(map[int64]int64, 2)
	for rows.Next() {
		var albumID, artistID int64
		if err := rows.Scan(&albumID, &artistID); err != nil {
			rows.Close()
			return fmt.Errorf("scanning album artist: %w", err)
		}
		artistIDByAlbum[albumID] = artistID
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	loserArtistID, ok := artistIDByAlbum[loserID]
	if !ok {
		return ErrAlbumNotFound
	}
	winnerArtistID, ok := artistIDByAlbum[winnerID]
	if !ok {
		return ErrAlbumNotFound
	}
	if loserArtistID != winnerArtistID {
		return ErrAlbumsBelongToDifferentArtists
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
	if err := dedupeSingleFlag(ctx, tx, albumCoverTable, winnerAlbumID); err != nil {
		return fmt.Errorf("deduping primary cover: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM albums WHERE id = $1`, loserAlbumID); err != nil {
		return fmt.Errorf("deleting merged album: %w", err)
	}
	return nil
}

// albumRow is queryAlbumTitles' result row: just enough to detect and act
// on a same-titled-album collision during MergeArtists.
type albumRow struct {
	id    int64
	title string
}

// queryAlbumTitles lists an artist's albums' IDs and titles, used by
// MergeArtists to detect same-titled albums that need folding together
// rather than simply reassigned.
func queryAlbumTitles(ctx context.Context, tx *sql.Tx, artistID int64) ([]albumRow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, title FROM albums WHERE artist_id = $1`, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []albumRow
	for rows.Next() {
		var a albumRow
		if err := rows.Scan(&a.id, &a.title); err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

// dedupeSingleFlag ensures at most one row in t's table (scoped by
// t.fkColumn = fkID) has is_primary = true, keeping whichever already was
// (oldest on a tie) and clearing the rest. Needed after a merge: two
// independently-primary galleries colliding into one row's gallery would
// otherwise leave two rows flagged true at once, breaking the "exactly
// one primary" invariant TDR 014 already relies on elsewhere
// (setGalleryPrimary, deleteGalleryRow's promotion).
func dedupeSingleFlag(ctx context.Context, tx *sql.Tx, t galleryTable, fkID int64) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE %s SET is_primary = FALSE WHERE %s = $1 AND is_primary = TRUE AND id != (
			SELECT id FROM %s WHERE %s = $1 AND is_primary = TRUE ORDER BY created_at ASC, id ASC LIMIT 1
		)
	`, t.name, t.fkColumn, t.name, t.fkColumn), fkID)
	if err != nil {
		return fmt.Errorf("deduping %s.is_primary: %w", t.name, err)
	}
	return nil
}
