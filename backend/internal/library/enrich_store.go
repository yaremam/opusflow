package library

import (
	"context"
	"fmt"

	"github.com/lib/pq"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// pendingOrFailed is the SQL fragment shared by both enrichment-target
// queries below.
const pendingOrFailed = `IN ('pending', 'failed')`

// ArtistsNeedingEnrichment returns up to limit artists with at least one of
// art/facts/bio still pending or failed, oldest-added first. Empty-name
// ("Unknown Artist") rows are excluded — they're seeded not_found by
// migration 0003, but this WHERE clause is what actually guarantees AC-3
// for artists created after that migration ran, since a fresh upsert always
// starts every status at the column default, 'pending'. Not scoped to any
// particular scan (TDR 003) — this is the same query on every run, which is
// what lets one job double as both post-scan enrichment and backfill for
// libraries that predate this feature.
func (s *Store) ArtistsNeedingEnrichment(ctx context.Context, limit int) ([]enrich.ArtistTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, COALESCE(musicbrainz_id, ''), art_status, facts_status, bio_status
		FROM artists
		WHERE name != ''
		  AND (art_status `+pendingOrFailed+` OR facts_status `+pendingOrFailed+` OR bio_status `+pendingOrFailed+`)
		ORDER BY id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing artists needing enrichment: %w", err)
	}
	defer rows.Close()

	var targets []enrich.ArtistTarget
	for rows.Next() {
		var t enrich.ArtistTarget
		if err := rows.Scan(&t.ID, &t.Name, &t.MusicBrainzID, &t.ArtStatus, &t.FactsStatus, &t.BioStatus); err != nil {
			return nil, fmt.Errorf("scanning artist enrich target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// AlbumsNeedingEnrichment is ArtistsNeedingEnrichment's album-flavored
// counterpart; see its doc comment.
func (s *Store) AlbumsNeedingEnrichment(ctx context.Context, limit int) ([]enrich.AlbumTarget, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT al.id, al.title, ar.name, COALESCE(al.musicbrainz_id, ''),
		       al.art_status, al.facts_status, al.description_status
		FROM albums al
		JOIN artists ar ON ar.id = al.artist_id
		WHERE al.title != ''
		  AND (al.art_status `+pendingOrFailed+` OR al.facts_status `+pendingOrFailed+` OR al.description_status `+pendingOrFailed+`)
		ORDER BY al.id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing albums needing enrichment: %w", err)
	}
	defer rows.Close()

	var targets []enrich.AlbumTarget
	for rows.Next() {
		var t enrich.AlbumTarget
		if err := rows.Scan(&t.ID, &t.Title, &t.ArtistName, &t.MusicBrainzID, &t.ArtStatus, &t.FactsStatus, &t.DescriptionStatus); err != nil {
			return nil, fmt.Errorf("scanning album enrich target: %w", err)
		}
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// SetArtistMusicBrainzID records the MBID an artist search matched, so
// later enrichment attempts for the same artist look it up directly
// instead of searching by name again.
func (s *Store) SetArtistMusicBrainzID(ctx context.Context, id int64, mbid string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE artists SET musicbrainz_id = $2 WHERE id = $1`, id, mbid)
	if err != nil {
		return fmt.Errorf("setting artist musicbrainz id: %w", err)
	}
	return nil
}

// SetArtistArt records the outcome of an artist photo lookup. Only a
// Found outcome overwrites the path columns — a NotFound/Failed write
// touches art_status alone, so a previously-found photo survives a later
// attempt (e.g. a manual retry, TDR 007) that doesn't turn up a
// replacement, rather than being nulled out by it.
func (s *Store) SetArtistArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE artists SET
			art_status = $2,
			photo_thumb_path = CASE WHEN $5 = 'found' THEN $3 ELSE photo_thumb_path END,
			photo_path = CASE WHEN $5 = 'found' THEN $4 ELSE photo_path END
		WHERE id = $1
	`, id, status, nullIfEmpty(thumbPath), nullIfEmpty(fullPath), string(status))
	if err != nil {
		return fmt.Errorf("setting artist art: %w", err)
	}
	return nil
}

// ResetArtistArt marks an artist's art status pending again — used by a
// manual retry (TDR 007) to wake the enrichment job for just this one
// artist without touching any existing photo path, so the last known-good
// image keeps rendering until (and unless) a new one replaces it.
func (s *Store) ResetArtistArt(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE artists SET art_status = 'pending' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("resetting artist art status: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("resetting artist art status: %w", err)
	} else if n == 0 {
		return ErrArtistNotFound
	}
	return nil
}

// SetArtistFacts records the outcome of an artist facts lookup, taking the
// already-fetched info wholesale rather than one parameter per column —
// callers (Job) already have an enrich.ArtistInfo in hand, and every field
// this method reads from it is a column this table already has, so there's
// no translation to keep in sync beyond this one function. info.Genres may
// be nil (Job passes a zero-value ArtistInfo for an unresolved/failed
// lookup) — coerced to an empty slice so pq.Array never hands Postgres a
// NULL for the NOT NULL genres column; a nil Go slice and "no genres" are
// the same fact.
func (s *Store) SetArtistFacts(ctx context.Context, id int64, status enrich.Status, info enrich.ArtistInfo) error {
	genres := info.Genres
	if genres == nil {
		genres = []string{}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE artists SET facts_status = $2, formed_year = $3, country = $4, genres = $5 WHERE id = $1
	`, id, status, info.FormedYear, info.Country, pq.Array(genres))
	if err != nil {
		return fmt.Errorf("setting artist facts: %w", err)
	}
	return nil
}

// SetArtistBio records the outcome of an artist bio lookup.
func (s *Store) SetArtistBio(ctx context.Context, id int64, status enrich.Status, bio, sourceURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE artists SET bio_status = $2, bio = $3, bio_source_url = $4 WHERE id = $1
	`, id, status, bio, sourceURL)
	if err != nil {
		return fmt.Errorf("setting artist bio: %w", err)
	}
	return nil
}

// SetAlbumMusicBrainzID is SetArtistMusicBrainzID's album counterpart.
func (s *Store) SetAlbumMusicBrainzID(ctx context.Context, id int64, mbid string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE albums SET musicbrainz_id = $2 WHERE id = $1`, id, mbid)
	if err != nil {
		return fmt.Errorf("setting album musicbrainz id: %w", err)
	}
	return nil
}

// SetAlbumArt records the outcome of an album cover lookup. See
// SetArtistArt's doc comment — same Found-only path-column write, same
// reasoning.
func (s *Store) SetAlbumArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE albums SET
			art_status = $2,
			cover_thumb_path = CASE WHEN $5 = 'found' THEN $3 ELSE cover_thumb_path END,
			cover_path = CASE WHEN $5 = 'found' THEN $4 ELSE cover_path END
		WHERE id = $1
	`, id, status, nullIfEmpty(thumbPath), nullIfEmpty(fullPath), string(status))
	if err != nil {
		return fmt.Errorf("setting album art: %w", err)
	}
	return nil
}

// ResetAlbumArt is ResetArtistArt's album counterpart. See its doc comment.
func (s *Store) ResetAlbumArt(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE albums SET art_status = 'pending' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("resetting album art status: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("resetting album art status: %w", err)
	} else if n == 0 {
		return ErrAlbumNotFound
	}
	return nil
}

// SetAlbumArtIfOpen is SetAlbumArt, but only takes effect while the
// album's art is still open (pending or failed) — used for embedded-tag
// art (AC-1), where "first image found wins" must never clobber art a
// different track, or the enrichment job, already resolved.
func (s *Store) SetAlbumArtIfOpen(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE albums SET art_status = $2, cover_thumb_path = $3, cover_path = $4
		WHERE id = $1 AND art_status `+pendingOrFailed+`
	`, id, status, nullIfEmpty(thumbPath), nullIfEmpty(fullPath))
	if err != nil {
		return fmt.Errorf("setting album art if open: %w", err)
	}
	return nil
}

// SetAlbumFacts records the outcome of an album facts lookup, taking the
// already-fetched info wholesale. See SetArtistFacts's doc comment — same
// reasoning, same nil-genres handling.
func (s *Store) SetAlbumFacts(ctx context.Context, id int64, status enrich.Status, info enrich.ReleaseGroupInfo) error {
	genres := info.Genres
	if genres == nil {
		genres = []string{}
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE albums SET facts_status = $2, label = $3, country = $4, genres = $5 WHERE id = $1
	`, id, status, info.Label, info.Country, pq.Array(genres))
	if err != nil {
		return fmt.Errorf("setting album facts: %w", err)
	}
	return nil
}

// SetAlbumDescription records the outcome of an album description lookup.
func (s *Store) SetAlbumDescription(ctx context.Context, id int64, status enrich.Status, description, sourceURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE albums SET description_status = $2, description = $3, description_source_url = $4 WHERE id = $1
	`, id, status, description, sourceURL)
	if err != nil {
		return fmt.Errorf("setting album description: %w", err)
	}
	return nil
}

// nullIfEmpty turns "" into a SQL NULL so path columns stay genuinely NULL
// (never set) rather than an empty string, matching artistEnrichCols'/
// albumEnrichCols' COALESCE(..., ”) read side.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
