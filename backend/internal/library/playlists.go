package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// ErrPlaylistNotFound is returned when a playlist ID doesn't match any
// recorded playlist.
var ErrPlaylistNotFound = errors.New("playlist not found")

// Playlist is a household-shared, user-created ordered collection of
// tracks (TDR 028) — no per-user ownership, matching every other
// collection in this app (there's no identity/profile system yet).
type Playlist struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	TrackCount int       `json:"trackCount"`
	CreatedAt  time.Time `json:"createdAt"`
	// CoverURLs is up to 4 thumbnail URLs, oldest-position-first, from
	// its first tracks' albums — empty when the playlist has no tracks,
	// and "" for any of those four whose album has no cover yet.
	CoverURLs []string `json:"coverUrls"`
}

// PlaylistDetail is a single playlist plus its full, ordered track
// listing.
type PlaylistDetail struct {
	Playlist
	Tracks []PlaylistTrack `json:"tracks"`
}

// PlaylistTrack is one entry in a playlist — addressed by its own
// PlaylistTrackID (the playlist_tracks row), not TrackID, so the same
// track can appear more than once (AC-6) and each occurrence still be
// individually removable/reorderable.
type PlaylistTrack struct {
	PlaylistTrackID    int64  `json:"playlistTrackId"`
	TrackID            int64  `json:"trackId"`
	Title              string `json:"title"`
	ArtistName         string `json:"artistName"`
	AlbumTitle         string `json:"albumTitle"`
	AlbumCoverThumbURL string `json:"albumCoverThumbUrl"`
	DurationSeconds    int    `json:"durationSeconds"`
	Format             string `json:"format"`
}

// CreatePlaylist records a new, empty playlist.
func (s *Store) CreatePlaylist(ctx context.Context, name string) (Playlist, error) {
	var pl Playlist
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO playlists (name) VALUES ($1)
		RETURNING id, name, created_at
	`, name).Scan(&pl.ID, &pl.Name, &pl.CreatedAt)
	if err != nil {
		return Playlist{}, fmt.Errorf("creating playlist: %w", err)
	}
	pl.CoverURLs = []string{}
	return pl, nil
}

// ListPlaylists returns a page of playlists, newest or alphabetical
// first per opts.Sort — no genre/year filters, playlists don't have them.
func (s *Store) ListPlaylists(ctx context.Context, opts ListOptions) (Page[Playlist], error) {
	order := recentOrName(opts, "p.created_at", "p.name")
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.created_at,
		       COALESCE((SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id), 0),
		       COUNT(*) OVER()
		FROM playlists p
		ORDER BY `+order+`
		LIMIT $1 OFFSET $2
	`, opts.PageSize, opts.offset())
	if err != nil {
		return Page[Playlist]{}, fmt.Errorf("listing playlists: %w", err)
	}
	defer rows.Close()

	page := Page[Playlist]{Page: opts.Page, PageSize: opts.PageSize, Items: []Playlist{}}
	ids := []int64{}
	for rows.Next() {
		var pl Playlist
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.CreatedAt, &pl.TrackCount, &page.TotalCount); err != nil {
			return Page[Playlist]{}, fmt.Errorf("scanning playlist: %w", err)
		}
		pl.CoverURLs = []string{}
		page.Items = append(page.Items, pl)
		ids = append(ids, pl.ID)
	}
	if err := rows.Err(); err != nil {
		return Page[Playlist]{}, err
	}

	covers, err := s.playlistCoverURLs(ctx, ids)
	if err != nil {
		return Page[Playlist]{}, err
	}
	for i := range page.Items {
		page.Items[i].CoverURLs = covers[page.Items[i].ID]
	}
	return page, nil
}

// GetPlaylist fetches a single playlist plus its full ordered track
// listing.
func (s *Store) GetPlaylist(ctx context.Context, id int64) (PlaylistDetail, error) {
	var d PlaylistDetail
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id, p.name, p.created_at,
		       COALESCE((SELECT COUNT(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id), 0)
		FROM playlists p
		WHERE p.id = $1
	`, id).Scan(&d.ID, &d.Name, &d.CreatedAt, &d.TrackCount)
	if errors.Is(err, sql.ErrNoRows) {
		return PlaylistDetail{}, ErrPlaylistNotFound
	}
	if err != nil {
		return PlaylistDetail{}, fmt.Errorf("getting playlist: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT pt.id, t.id, t.title, t.artist, al.title, COALESCE(ac.thumb_path, ''), t.duration_seconds, t.path
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		LEFT JOIN albums al ON al.id = t.album_id
		`+trackAlbumPrimaryCoverJoin+`
		WHERE pt.playlist_id = $1
		ORDER BY pt.position ASC
	`, id)
	if err != nil {
		return PlaylistDetail{}, fmt.Errorf("listing playlist's tracks: %w", err)
	}
	defer rows.Close()

	d.Tracks = []PlaylistTrack{}
	for rows.Next() {
		var pt PlaylistTrack
		var path string
		if err := rows.Scan(&pt.PlaylistTrackID, &pt.TrackID, &pt.Title, &pt.ArtistName, &pt.AlbumTitle, &pt.AlbumCoverThumbURL, &pt.DurationSeconds, &path); err != nil {
			return PlaylistDetail{}, fmt.Errorf("scanning playlist track: %w", err)
		}
		pt.Format = TrackFormat(path)
		d.Tracks = append(d.Tracks, pt)
	}
	if err := rows.Err(); err != nil {
		return PlaylistDetail{}, err
	}

	covers, err := s.playlistCoverURLs(ctx, []int64{id})
	if err != nil {
		return PlaylistDetail{}, err
	}
	d.CoverURLs = covers[id]
	return d, nil
}

// playlistCoverURLs is AC-7's collage source: up to 4 thumbnail URLs per
// playlist ID, from its first (lowest-position) tracks' albums, batched
// across every ID in one query rather than one query per playlist.
// Positions stay contiguous from 0 (enforced by AddTrackToPlaylist/
// RemovePlaylistTrack/ReorderPlaylistTracks renumbering on every write),
// so "position < 4" is a valid, simple way to get the first four without
// a LIMIT-per-group subquery.
func (s *Store) playlistCoverURLs(ctx context.Context, playlistIDs []int64) (map[int64][]string, error) {
	// Every ID starts mapped to an empty (non-nil) slice — a playlist with
	// no tracks, or none with a cover yet, never contributes a row to the
	// query below, and Playlist.CoverURLs must come back as `[]`, not
	// `null`, over JSON (a nil slice crashes the web client's `.length`
	// check).
	result := make(map[int64][]string, len(playlistIDs))
	for _, id := range playlistIDs {
		result[id] = []string{}
	}
	if len(playlistIDs) == 0 {
		return result, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT pt.playlist_id, COALESCE(ac.thumb_path, '')
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		`+trackAlbumPrimaryCoverJoin+`
		WHERE pt.playlist_id = ANY($1) AND pt.position < 4
		ORDER BY pt.playlist_id, pt.position
	`, pq.Array(playlistIDs))
	if err != nil {
		return nil, fmt.Errorf("listing playlist cover art: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var playlistID int64
		var thumbURL string
		if err := rows.Scan(&playlistID, &thumbURL); err != nil {
			return nil, fmt.Errorf("scanning playlist cover art: %w", err)
		}
		result[playlistID] = append(result[playlistID], thumbURL)
	}
	return result, rows.Err()
}

// RenamePlaylist updates a playlist's name.
func (s *Store) RenamePlaylist(ctx context.Context, id int64, name string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE playlists SET name = $1 WHERE id = $2`, name, id)
	if err != nil {
		return fmt.Errorf("renaming playlist: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("renaming playlist: %w", err)
	}
	if n == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

// DeletePlaylist removes a playlist — cascades to its playlist_tracks
// rows, never touches the underlying tracks or files.
func (s *Store) DeletePlaylist(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting playlist: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("deleting playlist: %w", err)
	}
	if n == 0 {
		return ErrPlaylistNotFound
	}
	return nil
}

// AddTrackToPlaylist appends trackID to the end of the playlist — no
// dedup (AC-6), matching addToQueue's own rule for the in-memory queue.
func (s *Store) AddTrackToPlaylist(ctx context.Context, playlistID, trackID int64) (PlaylistTrack, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PlaylistTrack{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	var nextPosition int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(position) + 1, 0) FROM playlist_tracks WHERE playlist_id = $1
	`, playlistID).Scan(&nextPosition); err != nil {
		return PlaylistTrack{}, fmt.Errorf("computing next position: %w", err)
	}

	var pt PlaylistTrack
	err = tx.QueryRowContext(ctx, `
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		VALUES ($1, $2, $3)
		RETURNING id, track_id
	`, playlistID, trackID, nextPosition).Scan(&pt.PlaylistTrackID, &pt.TrackID)
	if err != nil {
		return PlaylistTrack{}, fmt.Errorf("adding track to playlist: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return PlaylistTrack{}, fmt.Errorf("committing add-to-playlist: %w", err)
	}
	return pt, nil
}

// RemovePlaylistTrack removes one entry (addressed by its own row ID, so
// a duplicate elsewhere in the playlist is unaffected) and renumbers the
// remaining entries so position stays contiguous from 0.
func (s *Store) RemovePlaylistTrack(ctx context.Context, playlistID, playlistTrackID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM playlist_tracks WHERE id = $1 AND playlist_id = $2
	`, playlistTrackID, playlistID); err != nil {
		return fmt.Errorf("removing playlist track: %w", err)
	}

	if err := renumberPlaylistPositions(ctx, tx, playlistID); err != nil {
		return err
	}

	return tx.Commit()
}

// ReorderPlaylistTracks moves the entry identified by playlistTrackID to
// toIndex (0-based) among its playlist's other entries, renumbering
// every affected row's position.
func (s *Store) ReorderPlaylistTracks(ctx context.Context, playlistID, playlistTrackID int64, toIndex int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM playlist_tracks WHERE playlist_id = $1 ORDER BY position ASC
	`, playlistID)
	if err != nil {
		return fmt.Errorf("listing playlist track order: %w", err)
	}
	var order []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scanning playlist track order: %w", err)
		}
		order = append(order, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	fromIndex := -1
	for i, id := range order {
		if id == playlistTrackID {
			fromIndex = i
			break
		}
	}
	if fromIndex == -1 {
		return fmt.Errorf("playlist track %d not found in playlist %d", playlistTrackID, playlistID)
	}
	if toIndex < 0 {
		toIndex = 0
	}
	if toIndex >= len(order) {
		toIndex = len(order) - 1
	}

	moved := order[fromIndex]
	order = append(order[:fromIndex], order[fromIndex+1:]...)
	order = append(order[:toIndex], append([]int64{moved}, order[toIndex:]...)...)

	// One set-based UPDATE across every row's new position, the same
	// pq.Array-batching idiom playlistCoverURLs already uses, rather than
	// a round trip per row.
	positions := make([]int32, len(order))
	for i := range order {
		positions[i] = int32(i)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE playlist_tracks pt
		SET position = v.position
		FROM unnest($1::bigint[], $2::int[]) AS v(id, position)
		WHERE pt.id = v.id AND pt.playlist_id = $3
	`, pq.Array(order), pq.Array(positions), playlistID); err != nil {
		return fmt.Errorf("updating playlist track positions: %w", err)
	}

	return tx.Commit()
}

// renumberPlaylistPositions closes any gap left by a removal so position
// stays contiguous from 0 — playlistCoverURLs' "position < 4" shortcut
// and ReorderPlaylistTracks' index arithmetic both depend on this.
func renumberPlaylistPositions(ctx context.Context, tx *sql.Tx, playlistID int64) error {
	if _, err := tx.ExecContext(ctx, `
		WITH renumbered AS (
			SELECT id, ROW_NUMBER() OVER (ORDER BY position ASC) - 1 AS new_position
			FROM playlist_tracks
			WHERE playlist_id = $1
		)
		UPDATE playlist_tracks pt
		SET position = renumbered.new_position
		FROM renumbered
		WHERE pt.id = renumbered.id
	`, playlistID); err != nil {
		return fmt.Errorf("renumbering playlist positions: %w", err)
	}
	return nil
}

// ListPlaylistsContainingTrack backs the "Add to playlist" picker's
// pre-checked state (AC-5) — every playlist with at least one entry
// pointing at trackID.
func (s *Store) ListPlaylistsContainingTrack(ctx context.Context, trackID int64) ([]Playlist, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.name, p.created_at
		FROM playlists p
		JOIN playlist_tracks pt ON pt.playlist_id = p.id
		WHERE pt.track_id = $1
		ORDER BY p.name ASC
	`, trackID)
	if err != nil {
		return nil, fmt.Errorf("listing playlists containing track: %w", err)
	}
	defer rows.Close()

	playlists := []Playlist{}
	for rows.Next() {
		var pl Playlist
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning playlist: %w", err)
		}
		pl.CoverURLs = []string{}
		playlists = append(playlists, pl)
	}
	return playlists, rows.Err()
}
