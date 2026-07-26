package enrich

import "context"

// Status is the state of one kind of background enrichment (art, facts, or
// bio/description) for a single artist or album. "not_found" is a
// terminal, accepted negative (the source genuinely has nothing) and is
// never retried; "failed" (a network/rate-limit error) is retried on the
// Job's next run. See TDR 003.
type Status string

const (
	Pending  Status = "pending"
	Found    Status = "found"
	NotFound Status = "not_found"
	Failed   Status = "failed"
)

// ArtistTarget is one artist Job still owes at least one of art/facts/bio
// to — either it's never been looked at (pending) or a prior attempt hit a
// transient error (failed). MusicBrainzID is empty until the first
// successful search match, after which Job reuses it instead of searching
// by name again.
type ArtistTarget struct {
	ID            int64
	Name          string
	MusicBrainzID string
	ArtStatus     Status
	FactsStatus   Status
	BioStatus     Status
}

// AlbumTarget is ArtistTarget's album-flavored counterpart.
type AlbumTarget struct {
	ID                int64
	Title             string
	ArtistName        string
	MusicBrainzID     string
	ArtStatus         Status
	FactsStatus       Status
	DescriptionStatus Status
}

// Store is the persistence Job needs — the subset of library.Store's
// methods it actually calls. *library.Store satisfies this as its one
// production adapter (structurally, the same pattern scan.ProgressStore
// uses); tests substitute an in-memory fake.
type Store interface {
	ArtistsNeedingEnrichment(ctx context.Context, limit int) ([]ArtistTarget, error)
	AlbumsNeedingEnrichment(ctx context.Context, limit int) ([]AlbumTarget, error)

	SetArtistMusicBrainzID(ctx context.Context, id int64, mbid string) error
	SetArtistArt(ctx context.Context, id int64, status Status, thumbPath, fullPath string) error
	SetArtistFacts(ctx context.Context, id int64, status Status, formedYear int, country string, genres []string) error
	SetArtistBio(ctx context.Context, id int64, status Status, bio, sourceURL string) error

	SetAlbumMusicBrainzID(ctx context.Context, id int64, mbid string) error
	SetAlbumArt(ctx context.Context, id int64, status Status, thumbPath, fullPath string) error
	SetAlbumFacts(ctx context.Context, id int64, status Status, label, country string, genres []string) error
	SetAlbumDescription(ctx context.Context, id int64, status Status, description, sourceURL string) error
}
