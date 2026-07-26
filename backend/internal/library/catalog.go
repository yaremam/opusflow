package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// ErrArtistNotFound is returned when an artist ID doesn't match any
// registered artist.
var ErrArtistNotFound = errors.New("artist not found")

// ErrAlbumNotFound is returned when an album ID doesn't match any
// registered album.
var ErrAlbumNotFound = errors.New("album not found")

// Artist is one artist attributed to at least one track in the library.
// Untagged tracks are attributed to a real "Unknown Artist" row (empty
// Name) rather than excluded from browsing.
//
// PhotoThumbURL/PhotoURL, FormedYear/Country/Genres, and Bio/BioSourceURL
// are populated by the background enrichment job (TDR 003) and start out
// zero-valued — an empty PhotoURL is the client's signal to render the
// placeholder tile; empty Genres/Bio mean that section simply isn't shown.
type Artist struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	AlbumCount int       `json:"albumCount"`
	TrackCount int       `json:"trackCount"`
	CreatedAt  time.Time `json:"createdAt"`

	PhotoThumbURL string        `json:"photoThumbUrl"`
	PhotoURL      string        `json:"photoUrl"`
	ArtStatus     enrich.Status `json:"artStatus"`
	FormedYear    int           `json:"formedYear"`
	Country       string        `json:"country"`
	Genres        []string      `json:"genres"`
	Bio           string        `json:"bio"`
	BioSourceURL  string        `json:"bioSourceUrl"`
}

// Album is one album attributed to a single Artist. See Artist's doc
// comment for the enrichment-field conventions (CoverThumbURL/CoverURL,
// Label/Country/Genres, Description/DescriptionSourceURL) — same idea,
// album-flavored.
type Album struct {
	ID         int64     `json:"id"`
	Title      string    `json:"title"`
	ArtistID   int64     `json:"artistId"`
	ArtistName string    `json:"artistName"`
	Year       int       `json:"year"`
	TrackCount int       `json:"trackCount"`
	CreatedAt  time.Time `json:"createdAt"`

	CoverThumbURL        string        `json:"coverThumbUrl"`
	CoverURL             string        `json:"coverUrl"`
	ArtStatus            enrich.Status `json:"artStatus"`
	Label                string        `json:"label"`
	Country              string        `json:"country"`
	Genres               []string      `json:"genres"`
	Description          string        `json:"description"`
	DescriptionSourceURL string        `json:"descriptionSourceUrl"`
}

// Song is one imported track, with its artist and album names denormalized
// alongside the IDs so a songs listing doesn't need a client-side join.
type Song struct {
	ID                 int64         `json:"id"`
	Title              string        `json:"title"`
	ArtistID           int64         `json:"artistId"`
	ArtistName         string        `json:"artistName"`
	AlbumID            int64         `json:"albumId"`
	AlbumTitle         string        `json:"albumTitle"`
	AlbumCoverThumbURL string        `json:"albumCoverThumbUrl"`
	AlbumArtStatus     enrich.Status `json:"albumArtStatus"`
	TrackNumber        int           `json:"trackNumber"`
	Year               int           `json:"year"`
	Genre              string        `json:"genre"`
	DurationSeconds    int           `json:"durationSeconds"`
	CreatedAt          time.Time     `json:"createdAt"`
}

// AlbumTrack is one track within an AlbumDetail's listing — narrower than
// Song since the album/artist it belongs to is already the page it's on.
type AlbumTrack struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	TrackNumber     int    `json:"trackNumber"`
	DurationSeconds int    `json:"durationSeconds"`
}

// ArtistDetail is a single artist plus every album attributed to them,
// newest first.
type ArtistDetail struct {
	Artist
	Albums []Album `json:"albums"`
}

// AlbumDetail is a single album plus its full track listing, ordered by
// track number.
type AlbumDetail struct {
	Album
	Tracks []AlbumTrack `json:"tracks"`
}

// ListOptions controls sorting, filtering, and pagination shared by
// ListArtists, ListAlbums, and ListSongs. Callers are expected to have
// already normalized these (page >= 1, a valid PageSize, Sort one of
// "recent"/"name") — see Service, which owns that normalization.
type ListOptions struct {
	Page     int
	PageSize int
	Sort     string // "recent" (default) or "name"
	Genre    string // "" = no filter
	Year     int    // 0 = no filter
	Query    string // "" = no filter; matched case-insensitively, substring
}

// Page is one page of List* results, plus the total number of rows the
// filter matched before pagination was applied.
type Page[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	TotalCount int `json:"totalCount"`
}

func (o ListOptions) offset() int { return (o.Page - 1) * o.PageSize }

// artistEnrichCols/albumEnrichCols are the enrichment columns (TDR 003)
// shared by every query that returns a full Artist/Album — path columns are
// nullable (never set until the enrich job first runs) and COALESCEd to ”
// here so Go's Artist/Album never need a nullable string type, matching
// this codebase's existing "real empty row, not null" convention.
const artistEnrichCols = `COALESCE(a.photo_thumb_path, ''), COALESCE(a.photo_path, ''), a.art_status, a.formed_year, a.country, a.genres, a.bio, a.bio_source_url`
const albumEnrichCols = `COALESCE(al.cover_thumb_path, ''), COALESCE(al.cover_path, ''), al.art_status, al.label, al.country, al.genres, al.description, al.description_source_url`

// scanArtistEnrich/scanAlbumEnrich are the *sql.Row/*sql.Rows destinations
// matching artistEnrichCols/albumEnrichCols, in order.
func scanArtistEnrich(a *Artist) []any {
	return []any{&a.PhotoThumbURL, &a.PhotoURL, &a.ArtStatus, &a.FormedYear, &a.Country, pq.Array(&a.Genres), &a.Bio, &a.BioSourceURL}
}
func scanAlbumEnrich(al *Album) []any {
	return []any{&al.CoverThumbURL, &al.CoverURL, &al.ArtStatus, &al.Label, &al.Country, pq.Array(&al.Genres), &al.Description, &al.DescriptionSourceURL}
}

// recentOrName picks the literal ORDER BY fragment for opts.Sort. Sort is
// always one of a small fixed set validated by Service before it reaches
// here, so building the clause this way (rather than parameterizing it) is
// safe — it never carries request-controlled text into the query string.
func recentOrName(opts ListOptions, recentCol, nameCol string) string {
	if opts.Sort == "name" {
		return nameCol + " ASC, id ASC"
	}
	return recentCol + " DESC, id DESC"
}

// upsertArtist finds-or-creates the artist row named name, returning its
// ID. The "DO UPDATE SET name = EXCLUDED.name" no-op is what makes RETURNING
// work on the conflict path too — a plain "DO NOTHING" returns no row at
// all when the artist already exists.
func upsertArtist(ctx context.Context, q queryer, name string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO artists (name) VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upserting artist: %w", err)
	}
	return id, nil
}

// upsertAlbum finds-or-creates the (title, artistID) album row, returning
// its ID. year is only recorded on first insert — if a later track for the
// same album carries a different year (rare tagging inconsistency), the
// album keeps whichever year it was first created with.
func upsertAlbum(ctx context.Context, q queryer, title string, artistID int64, year int) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `
		INSERT INTO albums (title, artist_id, year) VALUES ($1, $2, $3)
		ON CONFLICT (title, artist_id) DO UPDATE SET title = EXCLUDED.title
		RETURNING id
	`, title, artistID, year).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upserting album: %w", err)
	}
	return id, nil
}

// queryer is satisfied by both *sql.DB and *sql.Tx, so upsertArtist/
// upsertAlbum work identically whether called standalone or inside a
// transaction.
type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ListArtists returns a page of artists matching opts.
func (s *Store) ListArtists(ctx context.Context, opts ListOptions) (Page[Artist], error) {
	order := recentOrName(opts, "a.created_at", "a.name")
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.name, a.created_at,
		       (SELECT COUNT(*) FROM albums al WHERE al.artist_id = a.id),
		       (SELECT COUNT(*) FROM tracks t WHERE t.artist_id = a.id),
		       `+artistEnrichCols+`,
		       COUNT(*) OVER()
		FROM artists a
		WHERE ($3 = '' OR EXISTS (SELECT 1 FROM tracks t WHERE t.artist_id = a.id AND t.genre ILIKE '%' || $3 || '%'))
		  AND ($4 = 0 OR EXISTS (SELECT 1 FROM tracks t WHERE t.artist_id = a.id AND t.year = $4))
		  AND ($5 = '' OR a.name ILIKE '%' || $5 || '%')
		ORDER BY `+order+`
		LIMIT $1 OFFSET $2
	`, opts.PageSize, opts.offset(), opts.Genre, opts.Year, opts.Query)
	if err != nil {
		return Page[Artist]{}, fmt.Errorf("listing artists: %w", err)
	}
	defer rows.Close()

	page := Page[Artist]{Page: opts.Page, PageSize: opts.PageSize, Items: []Artist{}}
	for rows.Next() {
		var a Artist
		dest := append([]any{&a.ID, &a.Name, &a.CreatedAt, &a.AlbumCount, &a.TrackCount}, scanArtistEnrich(&a)...)
		dest = append(dest, &page.TotalCount)
		if err := rows.Scan(dest...); err != nil {
			return Page[Artist]{}, fmt.Errorf("scanning artist: %w", err)
		}
		page.Items = append(page.Items, a)
	}
	if err := rows.Err(); err != nil {
		return Page[Artist]{}, err
	}
	return page, nil
}

// GetArtist fetches a single artist by ID, with every album attributed to
// them, newest first.
func (s *Store) GetArtist(ctx context.Context, id int64) (ArtistDetail, error) {
	var d ArtistDetail
	dest := append([]any{&d.ID, &d.Name, &d.CreatedAt, &d.AlbumCount, &d.TrackCount}, scanArtistEnrich(&d.Artist)...)
	err := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.name, a.created_at,
		       (SELECT COUNT(*) FROM albums al WHERE al.artist_id = a.id),
		       (SELECT COUNT(*) FROM tracks t WHERE t.artist_id = a.id),
		       `+artistEnrichCols+`
		FROM artists a WHERE a.id = $1
	`, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ArtistDetail{}, ErrArtistNotFound
	}
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("getting artist: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT al.id, al.title, al.artist_id, a.name, al.year, al.created_at,
		       (SELECT COUNT(*) FROM tracks t WHERE t.album_id = al.id),
		       `+albumEnrichCols+`
		FROM albums al
		JOIN artists a ON a.id = al.artist_id
		WHERE al.artist_id = $1
		ORDER BY al.year DESC, al.title ASC
	`, id)
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("listing artist's albums: %w", err)
	}
	defer rows.Close()

	d.Albums = []Album{}
	for rows.Next() {
		var al Album
		dest := append([]any{&al.ID, &al.Title, &al.ArtistID, &al.ArtistName, &al.Year, &al.CreatedAt, &al.TrackCount}, scanAlbumEnrich(&al)...)
		if err := rows.Scan(dest...); err != nil {
			return ArtistDetail{}, fmt.Errorf("scanning album: %w", err)
		}
		d.Albums = append(d.Albums, al)
	}
	if err := rows.Err(); err != nil {
		return ArtistDetail{}, err
	}
	return d, nil
}

// ListAlbums returns a page of albums matching opts. Genre isn't a column
// on albums (only tracks carry it), so genre filtering matches albums with
// at least one track in that genre; year filters directly on the album's
// own year.
func (s *Store) ListAlbums(ctx context.Context, opts ListOptions) (Page[Album], error) {
	order := recentOrName(opts, "al.created_at", "al.title")
	rows, err := s.db.QueryContext(ctx, `
		SELECT al.id, al.title, al.artist_id, ar.name, al.year, al.created_at,
		       (SELECT COUNT(*) FROM tracks t WHERE t.album_id = al.id),
		       `+albumEnrichCols+`,
		       COUNT(*) OVER()
		FROM albums al
		JOIN artists ar ON ar.id = al.artist_id
		WHERE ($3 = '' OR EXISTS (SELECT 1 FROM tracks t WHERE t.album_id = al.id AND t.genre ILIKE '%' || $3 || '%'))
		  AND ($4 = 0 OR al.year = $4)
		  AND ($5 = '' OR al.title ILIKE '%' || $5 || '%' OR ar.name ILIKE '%' || $5 || '%')
		ORDER BY `+order+`
		LIMIT $1 OFFSET $2
	`, opts.PageSize, opts.offset(), opts.Genre, opts.Year, opts.Query)
	if err != nil {
		return Page[Album]{}, fmt.Errorf("listing albums: %w", err)
	}
	defer rows.Close()

	page := Page[Album]{Page: opts.Page, PageSize: opts.PageSize, Items: []Album{}}
	for rows.Next() {
		var al Album
		dest := append([]any{&al.ID, &al.Title, &al.ArtistID, &al.ArtistName, &al.Year, &al.CreatedAt, &al.TrackCount}, scanAlbumEnrich(&al)...)
		dest = append(dest, &page.TotalCount)
		if err := rows.Scan(dest...); err != nil {
			return Page[Album]{}, fmt.Errorf("scanning album: %w", err)
		}
		page.Items = append(page.Items, al)
	}
	if err := rows.Err(); err != nil {
		return Page[Album]{}, err
	}
	return page, nil
}

// GetAlbum fetches a single album by ID, with its full track listing
// ordered by track number.
func (s *Store) GetAlbum(ctx context.Context, id int64) (AlbumDetail, error) {
	var d AlbumDetail
	dest := append([]any{&d.ID, &d.Title, &d.ArtistID, &d.ArtistName, &d.Year, &d.CreatedAt, &d.TrackCount}, scanAlbumEnrich(&d.Album)...)
	err := s.db.QueryRowContext(ctx, `
		SELECT al.id, al.title, al.artist_id, ar.name, al.year, al.created_at,
		       (SELECT COUNT(*) FROM tracks t WHERE t.album_id = al.id),
		       `+albumEnrichCols+`
		FROM albums al
		JOIN artists ar ON ar.id = al.artist_id
		WHERE al.id = $1
	`, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return AlbumDetail{}, ErrAlbumNotFound
	}
	if err != nil {
		return AlbumDetail{}, fmt.Errorf("getting album: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, track_number, duration_seconds
		FROM tracks
		WHERE album_id = $1
		ORDER BY track_number ASC, title ASC
	`, id)
	if err != nil {
		return AlbumDetail{}, fmt.Errorf("listing album's tracks: %w", err)
	}
	defer rows.Close()

	d.Tracks = []AlbumTrack{}
	for rows.Next() {
		var t AlbumTrack
		if err := rows.Scan(&t.ID, &t.Title, &t.TrackNumber, &t.DurationSeconds); err != nil {
			return AlbumDetail{}, fmt.Errorf("scanning track: %w", err)
		}
		d.Tracks = append(d.Tracks, t)
	}
	if err := rows.Err(); err != nil {
		return AlbumDetail{}, err
	}
	return d, nil
}

// ListSongs returns a page of tracks matching opts.
func (s *Store) ListSongs(ctx context.Context, opts ListOptions) (Page[Song], error) {
	order := recentOrName(opts, "t.created_at", "t.title")
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.title, t.artist_id, ar.name, t.album_id, al.title,
		       COALESCE(al.cover_thumb_path, ''), al.art_status,
		       t.track_number, t.year, t.genre, t.duration_seconds, t.created_at,
		       COUNT(*) OVER()
		FROM tracks t
		JOIN artists ar ON ar.id = t.artist_id
		JOIN albums al ON al.id = t.album_id
		WHERE ($3 = '' OR t.genre ILIKE '%' || $3 || '%')
		  AND ($4 = 0 OR t.year = $4)
		  AND ($5 = '' OR t.title ILIKE '%' || $5 || '%' OR ar.name ILIKE '%' || $5 || '%' OR al.title ILIKE '%' || $5 || '%')
		ORDER BY `+order+`
		LIMIT $1 OFFSET $2
	`, opts.PageSize, opts.offset(), opts.Genre, opts.Year, opts.Query)
	if err != nil {
		return Page[Song]{}, fmt.Errorf("listing songs: %w", err)
	}
	defer rows.Close()

	page := Page[Song]{Page: opts.Page, PageSize: opts.PageSize, Items: []Song{}}
	for rows.Next() {
		var sg Song
		if err := rows.Scan(
			&sg.ID, &sg.Title, &sg.ArtistID, &sg.ArtistName, &sg.AlbumID, &sg.AlbumTitle, &sg.AlbumCoverThumbURL, &sg.AlbumArtStatus,
			&sg.TrackNumber, &sg.Year, &sg.Genre, &sg.DurationSeconds, &sg.CreatedAt, &page.TotalCount,
		); err != nil {
			return Page[Song]{}, fmt.Errorf("scanning song: %w", err)
		}
		page.Items = append(page.Items, sg)
	}
	if err := rows.Err(); err != nil {
		return Page[Song]{}, err
	}
	return page, nil
}
