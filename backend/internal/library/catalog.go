package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

// ErrSongNotFound is returned when a song (track) ID doesn't match any
// registered track.
var ErrSongNotFound = errors.New("song not found")

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
	Format             string        `json:"format"`
	BitrateKbps        int           `json:"bitrateKbps"`
}

// AlbumTrack is one track within an AlbumDetail's listing — narrower than
// Song since the album/artist it belongs to is already the page it's on.
type AlbumTrack struct {
	ID              int64  `json:"id"`
	Title           string `json:"title"`
	TrackNumber     int    `json:"trackNumber"`
	DurationSeconds int    `json:"durationSeconds"`
	Format          string `json:"format"`
	BitrateKbps     int    `json:"bitrateKbps"`
}

// bitrateKbps derives average bitrate (TDR 027) — file size in bits divided
// by duration — the one formula this app uses across every supported
// format, rather than parsing each format's own bitrate representation
// (several of which, like FLAC, don't have a single header field for it
// the way MP3 does). 0 means unknown: either an old track scanned before
// file_size_bytes existed, or a duration of 0 that would make the division
// meaningless.
func bitrateKbps(fileSizeBytes sql.NullInt64, durationSeconds int) int {
	if !fileSizeBytes.Valid || durationSeconds <= 0 {
		return 0
	}
	return int(fileSizeBytes.Int64 * 8 / int64(durationSeconds) / 1000)
}

// TrackFormat derives a track's format (TDR 015) from its on-disk path's
// extension — lowercased, dot stripped ("mp3"/"flac"/"m4a"/"ogg"/"wv").
// The path itself is never part of any API response; only this derived
// label is, the same privacy stance artwork's relative /artwork/ URLs
// already take toward ARTWORK_DIR's real filesystem location. Exported so
// httpserver's streaming handler can derive the same Content-Type from
// the same single notion of "format" this package already computes,
// rather than re-parsing the extension independently.
func TrackFormat(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

// ArtistDetail is a single artist plus every album attributed to them,
// newest first, and their full photo gallery (TDR 014). BannerURL is the
// detail page header's banner image (TDR 016) — the gallery photo flagged
// IsBanner, falling back to the primary photo when none is flagged yet,
// entirely via artistBannerJoin's ORDER BY; never blank while the gallery
// has any photo at all.
type ArtistDetail struct {
	Artist
	Albums    []Album       `json:"albums"`
	Photos    []ArtistPhoto `json:"photos"`
	BannerURL string        `json:"bannerUrl"`
}

// AlbumDetail is a single album plus its full track listing, ordered by
// track number, and its full cover gallery (TDR 014). BannerURL is TDR
// 016's header banner image — see ArtistDetail.BannerURL.
type AlbumDetail struct {
	Album
	Tracks    []AlbumTrack `json:"tracks"`
	Covers    []AlbumCover `json:"covers"`
	BannerURL string       `json:"bannerUrl"`
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
const artistEnrichCols = `COALESCE(ap.thumb_path, ''), COALESCE(ap.full_path, ''), a.art_status, a.formed_year, a.country, a.genres, a.bio, a.bio_source_url`
const albumEnrichCols = `COALESCE(ac.thumb_path, ''), COALESCE(ac.full_path, ''), al.art_status, al.label, al.country, al.genres, al.description, al.description_source_url`

// artistPrimaryPhotoJoin/albumPrimaryCoverJoin derive each artist's/
// album's single primary image (TDR 014's "exactly one primary per
// entity" invariant) for list views and tiles that only have room for
// one thumbnail — the full gallery is fetched separately via
// ListArtistPhotos/ListAlbumCovers. Every query using these must alias
// the artists row "a" (photo join) / albums row "al" (cover join).
const artistPrimaryPhotoJoin = `
	LEFT JOIN LATERAL (
		SELECT thumb_path, full_path FROM artist_photos
		WHERE artist_id = a.id
		ORDER BY is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	) ap ON true`
const albumPrimaryCoverJoin = `
	LEFT JOIN LATERAL (
		SELECT thumb_path, full_path FROM album_covers
		WHERE album_id = al.id
		ORDER BY is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	) ac ON true`

// trackAlbumPrimaryCoverJoin is albumPrimaryCoverJoin's sibling for a
// query that starts from a tracks row instead of an albums row (TDR
// 028's playlist queries — a playlist's track listing and its derived
// cover collage both need a track's own album's primary cover, with no
// reason to also join albums itself). Every query using this must alias
// the tracks row "t".
const trackAlbumPrimaryCoverJoin = `
	LEFT JOIN LATERAL (
		SELECT thumb_path FROM album_covers
		WHERE album_id = t.album_id
		ORDER BY is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	) ac ON true`

// artistBannerJoin/albumCoverBannerJoin derive the detail page header's
// banner image (TDR 016): the gallery image flagged is_banner if any,
// else falling back to the primary image, else the oldest — all in one
// ORDER BY rather than a separate join-plus-COALESCE, since "no banner
// chosen yet" (every existing gallery, until a reviewer picks one) must
// still render a real image, not a blank header. Detail-fetch only
// (GetArtist/GetAlbum), not the list/tile queries — a grid tile has no
// banner to show.
const artistBannerJoin = `
	LEFT JOIN LATERAL (
		SELECT full_path FROM artist_photos
		WHERE artist_id = a.id
		ORDER BY is_banner DESC, is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	) abn ON true`
const albumCoverBannerJoin = `
	LEFT JOIN LATERAL (
		SELECT full_path FROM album_covers
		WHERE album_id = al.id
		ORDER BY is_banner DESC, is_primary DESC, created_at ASC, id ASC
		LIMIT 1
	) acbn ON true`

// scanArtistEnrich/scanAlbumEnrich are the *sql.Row/*sql.Rows destinations
// matching artistEnrichCols/albumEnrichCols, in order.
func scanArtistEnrich(a *Artist) []any {
	return []any{&a.PhotoThumbURL, &a.PhotoURL, &a.ArtStatus, &a.FormedYear, &a.Country, pq.Array(&a.Genres), &a.Bio, &a.BioSourceURL}
}
func scanAlbumEnrich(al *Album) []any {
	return []any{&al.CoverThumbURL, &al.CoverURL, &al.ArtStatus, &al.Label, &al.Country, pq.Array(&al.Genres), &al.Description, &al.DescriptionSourceURL}
}

// artistCoreCols/albumCoreCols are the identity-plus-count columns every
// query returning a full Artist/Album selects before that query's own
// enrichment columns and whatever else it appends (a window count, a
// banner URL) — factored out so ListArtists/GetArtist (and ListAlbums/
// GetArtist's own album listing/GetAlbum) share one column list instead of
// three/four independently retyped copies drifting out of sync with each
// other. scanArtistCore/scanAlbumCore are their paired Scan destinations,
// in the same order — see scanArtistEnrich/scanAlbumEnrich above for the
// enrichment half of the same pattern.
const artistCoreCols = `a.id, a.name, a.created_at,
	       (SELECT COUNT(*) FROM albums al WHERE al.artist_id = a.id),
	       (SELECT COUNT(*) FROM tracks t WHERE t.artist_id = a.id)`

const albumCoreCols = `al.id, al.title, al.artist_id, ar.name, al.year, al.created_at,
	       (SELECT COUNT(*) FROM tracks t WHERE t.album_id = al.id)`

// albumFromJoin is the FROM/JOIN every query returning a full Album row
// starts from — al aliases albums, ar its artist, matching albumCoreCols/
// albumEnrichCols/albumPrimaryCoverJoin/albumCoverBannerJoin's own aliases.
const albumFromJoin = `FROM albums al JOIN artists ar ON ar.id = al.artist_id`

func scanArtistCore(a *Artist) []any {
	return []any{&a.ID, &a.Name, &a.CreatedAt, &a.AlbumCount, &a.TrackCount}
}
func scanAlbumCore(al *Album) []any {
	return []any{&al.ID, &al.Title, &al.ArtistID, &al.ArtistName, &al.Year, &al.CreatedAt, &al.TrackCount}
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
		SELECT `+artistCoreCols+`,
		       `+artistEnrichCols+`,
		       COUNT(*) OVER()
		FROM artists a`+artistPrimaryPhotoJoin+`
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
		dest := append(scanArtistCore(&a), scanArtistEnrich(&a)...)
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
	dest := append(scanArtistCore(&d.Artist), scanArtistEnrich(&d.Artist)...)
	dest = append(dest, &d.BannerURL)
	err := s.db.QueryRowContext(ctx, `
		SELECT `+artistCoreCols+`,
		       `+artistEnrichCols+`,
		       COALESCE(abn.full_path, '')
		FROM artists a`+artistPrimaryPhotoJoin+artistBannerJoin+`
		WHERE a.id = $1
	`, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ArtistDetail{}, ErrArtistNotFound
	}
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("getting artist: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT `+albumCoreCols+`,
		       `+albumEnrichCols+`
		`+albumFromJoin+albumPrimaryCoverJoin+`
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
		dest := append(scanAlbumCore(&al), scanAlbumEnrich(&al)...)
		if err := rows.Scan(dest...); err != nil {
			return ArtistDetail{}, fmt.Errorf("scanning album: %w", err)
		}
		d.Albums = append(d.Albums, al)
	}
	if err := rows.Err(); err != nil {
		return ArtistDetail{}, err
	}

	photos, err := s.ListArtistPhotos(ctx, id)
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("listing artist's photos: %w", err)
	}
	d.Photos = photos
	return d, nil
}

// ListAlbums returns a page of albums matching opts. Genre isn't a column
// on albums (only tracks carry it), so genre filtering matches albums with
// at least one track in that genre; year filters directly on the album's
// own year.
func (s *Store) ListAlbums(ctx context.Context, opts ListOptions) (Page[Album], error) {
	order := recentOrName(opts, "al.created_at", "al.title")
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+albumCoreCols+`,
		       `+albumEnrichCols+`,
		       COUNT(*) OVER()
		`+albumFromJoin+albumPrimaryCoverJoin+`
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
		dest := append(scanAlbumCore(&al), scanAlbumEnrich(&al)...)
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
	dest := append(scanAlbumCore(&d.Album), scanAlbumEnrich(&d.Album)...)
	dest = append(dest, &d.BannerURL)
	err := s.db.QueryRowContext(ctx, `
		SELECT `+albumCoreCols+`,
		       `+albumEnrichCols+`,
		       COALESCE(acbn.full_path, '')
		`+albumFromJoin+albumPrimaryCoverJoin+albumCoverBannerJoin+`
		WHERE al.id = $1
	`, id).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return AlbumDetail{}, ErrAlbumNotFound
	}
	if err != nil {
		return AlbumDetail{}, fmt.Errorf("getting album: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, track_number, duration_seconds, path, file_size_bytes
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
		var path string
		var fileSizeBytes sql.NullInt64
		if err := rows.Scan(&t.ID, &t.Title, &t.TrackNumber, &t.DurationSeconds, &path, &fileSizeBytes); err != nil {
			return AlbumDetail{}, fmt.Errorf("scanning track: %w", err)
		}
		t.Format = TrackFormat(path)
		t.BitrateKbps = bitrateKbps(fileSizeBytes, t.DurationSeconds)
		d.Tracks = append(d.Tracks, t)
	}
	if err := rows.Err(); err != nil {
		return AlbumDetail{}, err
	}

	covers, err := s.ListAlbumCovers(ctx, id)
	if err != nil {
		return AlbumDetail{}, fmt.Errorf("listing album's covers: %w", err)
	}
	d.Covers = covers
	return d, nil
}

// ListSongs returns a page of tracks matching opts.
func (s *Store) ListSongs(ctx context.Context, opts ListOptions) (Page[Song], error) {
	order := recentOrName(opts, "t.created_at", "t.title")
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.title, t.artist_id, ar.name, t.album_id, al.title,
		       COALESCE(ac.thumb_path, ''), al.art_status,
		       t.track_number, t.year, t.genre, t.duration_seconds, t.created_at, t.path, t.file_size_bytes,
		       COUNT(*) OVER()
		FROM tracks t
		JOIN artists ar ON ar.id = t.artist_id
		JOIN albums al ON al.id = t.album_id`+albumPrimaryCoverJoin+`
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
		var path string
		var fileSizeBytes sql.NullInt64
		if err := rows.Scan(
			&sg.ID, &sg.Title, &sg.ArtistID, &sg.ArtistName, &sg.AlbumID, &sg.AlbumTitle, &sg.AlbumCoverThumbURL, &sg.AlbumArtStatus,
			&sg.TrackNumber, &sg.Year, &sg.Genre, &sg.DurationSeconds, &sg.CreatedAt, &path, &fileSizeBytes, &page.TotalCount,
		); err != nil {
			return Page[Song]{}, fmt.Errorf("scanning song: %w", err)
		}
		sg.Format = TrackFormat(path)
		sg.BitrateKbps = bitrateKbps(fileSizeBytes, sg.DurationSeconds)
		page.Items = append(page.Items, sg)
	}
	if err := rows.Err(); err != nil {
		return Page[Song]{}, err
	}
	return page, nil
}

// GetSongPath resolves id to its on-disk path (TDR 015) — used only by
// the audio-streaming handler; the path itself is never part of any List/
// Get response (see trackFormat's doc comment).
func (s *Store) GetSongPath(ctx context.Context, id int64) (string, error) {
	var path string
	err := s.db.QueryRowContext(ctx, `SELECT path FROM tracks WHERE id = $1`, id).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrSongNotFound
	}
	if err != nil {
		return "", fmt.Errorf("getting song path: %w", err)
	}
	return path, nil
}
