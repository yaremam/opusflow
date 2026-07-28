package library

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// Copier executes a confirmed plan, copying every track to its destination
// and recording the result. The real implementation is organize.CopyJob;
// tests substitute a fake.
type Copier interface {
	Copy(ctx context.Context, store organize.Store, importID int64, plan organize.Plan) organize.RunSummary
}

// Enricher runs one pass of the background artwork/info enrichment job
// (TDR 003) — processing every artist/album still owed art, facts, or
// bio/description, not just ones touched by the import that triggered it.
// The real implementation is enrich.Job; tests substitute a fake.
type Enricher interface {
	Run(ctx context.Context) enrich.RunSummary
}

// MetadataSearch is the interactive counterpart to Enricher (TDR 012):
// searching/browsing MusicBrainz on demand for the review screen's "Look
// up metadata" flow, rather than the background job's silent by-ID
// lookups. The real implementation is *enrich.MusicBrainz; tests
// substitute a fake.
type MetadataSearch interface {
	SearchArtists(ctx context.Context, name string) ([]enrich.ArtistMatch, error)
	ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]enrich.ReleaseGroupMatch, error)
	ReleaseGroupReleases(ctx context.Context, releaseGroupMBID string) ([]enrich.ReleaseMatch, error)
	ReleaseTracks(ctx context.Context, releaseMBID string) ([]enrich.Track, error)
}

// GalleryStore is the artist/album image-gallery persistence Service needs
// (TDR 014) — kept as its own interface so a test (or future caller) that
// only cares about gallery behavior depends on, and fakes, just this
// eight-method slice rather than ImportStore's full width.
type GalleryStore interface {
	ListArtistPhotos(ctx context.Context, artistID int64) ([]ArtistPhoto, error)
	AddArtistPhoto(ctx context.Context, artistID int64, thumbURL, fullURL, source, contentHash string) (ArtistPhoto, error)
	SetArtistPrimaryPhoto(ctx context.Context, artistID, photoID int64) error
	DeleteArtistPhoto(ctx context.Context, artistID, photoID int64) (thumbPath, fullPath string, err error)

	ListAlbumCovers(ctx context.Context, albumID int64) ([]AlbumCover, error)
	AddAlbumCover(ctx context.Context, albumID int64, thumbURL, fullURL, source, pictureType, contentHash string) (AlbumCover, error)
	SetAlbumPrimaryCover(ctx context.Context, albumID, coverID int64) error
	DeleteAlbumCover(ctx context.Context, albumID, coverID int64) (thumbPath, fullPath string, err error)
}

// EnrichmentStore is the background artwork-job persistence Service needs
// (TDR 003/007) — resetting an entity's art status for a retry, and
// recording a lookup's outcome (which, on Found, adds into the gallery
// above as a side effect; see SetArtistArt/SetAlbumArt's own doc comments).
type EnrichmentStore interface {
	ResetArtistArt(ctx context.Context, id int64) error
	ResetAlbumArt(ctx context.Context, id int64) error
	SetArtistArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error
	SetAlbumArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error
}

// CatalogReader is the artist/album/song browsing and direct-deletion
// persistence Service needs (TDR 002/005) — listing/paginating the
// catalog those imports populate, and removing an entity from it.
type CatalogReader interface {
	DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error
	DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error

	ListArtists(ctx context.Context, opts ListOptions) (Page[Artist], error)
	GetArtist(ctx context.Context, id int64) (ArtistDetail, error)
	ListAlbums(ctx context.Context, opts ListOptions) (Page[Album], error)
	GetAlbum(ctx context.Context, id int64) (AlbumDetail, error)
	ListSongs(ctx context.Context, opts ListOptions) (Page[Song], error)
	GetSongPath(ctx context.Context, id int64) (string, error)
}

// ImportRecorder is library and import bookkeeping persistence (TDR 005/006),
// plus organize.Store's methods so it can be handed straight to a Copier.
type ImportRecorder interface {
	organize.Store

	CreateLibrary(ctx context.Context, name, rootPath string) (Library, error)
	ListLibraries(ctx context.Context) ([]Library, error)
	GetLibrary(ctx context.Context, id int64) (Library, error)
	DeleteLibrary(ctx context.Context, id int64, deleteFiles bool) error

	CreateImport(ctx context.Context, libraryID int64, sourceDescription string) (Import, error)
	GetImport(ctx context.Context, id int64) (Import, error)
	ListImports(ctx context.Context) ([]Import, error)
	MarkImportComplete(ctx context.Context, id int64) error
	MarkImportFailed(ctx context.Context, id int64, errMsg string) error
}

// ImportStore is the persistence Service actually depends on — the union
// of the four cohesion-scoped interfaces above. *Store satisfies it as its
// one production adapter; tests substitute an in-memory fake so
// orchestration logic (validation, copy-goroutine timing, list-options
// normalization) can be tested without a database. Splitting it into four
// named pieces doesn't change what a whole-Service test's fake has to
// implement (it still needs all of them), but it does mean each concern's
// method list is declared, and can be depended on, on its own, instead of
// one 32-method interface with a single production adapter — a seam that
// wasn't earning its width.
type ImportStore interface {
	GalleryStore
	EnrichmentStore
	CatalogReader
	ImportRecorder
}

// defaultPageSize/maxPageSize bound ListOptions.PageSize once normalized by
// normalizeListOptions — every List* endpoint shares the same pagination
// defaults and ceiling.
const (
	defaultPageSize = 30
	maxPageSize     = 100
)

// normalizeListOptions clamps Page/PageSize into range and defaults Sort to
// "recent" for anything other than the one other recognized value, "name" —
// so a request built from unvalidated query params can't hand the store a
// negative offset, an unbounded page size, or an unrecognized sort value.
func normalizeListOptions(opts ListOptions) ListOptions {
	if opts.Page < 1 {
		opts.Page = 1
	}
	if opts.PageSize <= 0 {
		opts.PageSize = defaultPageSize
	}
	if opts.PageSize > maxPageSize {
		opts.PageSize = maxPageSize
	}
	if opts.Sort != "name" {
		opts.Sort = "recent"
	}
	return opts
}

// ErrSourceInsideLibrary is returned when an import's chosen source
// directory is the same as, or nested inside, an existing library's root —
// TDR 006 AC-8's rule against importing a library into itself.
var ErrSourceInsideLibrary = errors.New("import source is inside a library's root")

// Service orchestrates multiple libraries (TDR 006) and organize-on-import
// (TDR 005): creating/listing/deleting libraries, browsing the filesystem to
// choose an import source or a new library's root, building and validating
// a plan against a chosen library's root, confirming it (which copies files
// in the background), and removing catalog entries.
type Service struct {
	store       ImportStore
	copier      Copier
	enricher    Enricher
	browseRoot  string
	images      *enrich.ImageStore
	musicBrainz MetadataSearch
}

// NewService builds a Service.
func NewService(store ImportStore, copier Copier) *Service {
	return &Service{store: store, copier: copier}
}

// SetEnricher wires up the background artwork/info job, run after every
// import's copy completes. Kept as a setter rather than a NewService
// parameter so existing callers (tests included) that don't need
// enrichment aren't forced to construct one; nil (the default) just means
// ConfirmImport's copy-completion hook is a no-op.
func (s *Service) SetEnricher(enricher Enricher) {
	s.enricher = enricher
}

// SetImages wires up manual artwork upload (TDR 007) with the same
// ImageStore the background enrichment job already saves into — kept as a
// setter, same reasoning as SetEnricher, so a deployment with ARTWORK_DIR
// unset simply can't upload (ErrArtworkNotConfigured) rather than needing a
// stub to construct a Service at all.
func (s *Service) SetImages(images *enrich.ImageStore) {
	s.images = images
}

// SetMusicBrainzSearch wires up the interactive metadata-lookup flow
// (TDR 012) with a MusicBrainz client — unlike SetEnricher/SetImages,
// this has no ARTWORK_DIR-style gate: a text search needs no image
// storage, so the caller constructs and wires this unconditionally. Kept
// as a setter, same reasoning as the others, so a Service can still be
// built without it (SearchArtists and friends then return
// ErrMetadataSearchNotConfigured).
func (s *Service) SetMusicBrainzSearch(ms MetadataSearch) {
	s.musicBrainz = ms
}

// SetBrowseRoot confines Browse and CreateLibrary to root (DATA_DIR).
// Kept as a setter, same as SetEnricher, so tests and any deployment that
// doesn't set DATA_DIR aren't forced to configure one; the zero value ""
// means unrestricted (TDR 006's original stance).
func (s *Service) SetBrowseRoot(root string) {
	s.browseRoot = root
}

// BrowseRoot returns the configured root (empty if unrestricted) — the web
// app's folder picker needs this to know where to start browsing instead of
// defaulting to "/" and immediately hitting ErrOutsideRoot.
func (s *Service) BrowseRoot() string {
	return s.browseRoot
}

// Browse lists the immediate subdirectories of path — used for both
// choosing an import source and choosing a new library's root. Rejects path
// if it falls outside the configured browse root (empty by default, see
// SetBrowseRoot).
func (s *Service) Browse(path string) ([]Entry, error) {
	if err := WithinRoot(path, s.browseRoot); err != nil {
		return nil, err
	}
	return Browse(path)
}

// CreateLibrary records a new library, rooted at rootPath (which must
// already exist as a directory, and fall within the configured browse root
// if one is set).
func (s *Service) CreateLibrary(ctx context.Context, name, rootPath string) (Library, error) {
	clean := filepath.Clean(rootPath)
	if err := WithinRoot(clean, s.browseRoot); err != nil {
		return Library{}, err
	}
	return s.store.CreateLibrary(ctx, name, clean)
}

// CreateFolder makes a new subdirectory named name inside parent — used by
// the create-library dialog's "new folder" affordance, so a library's root
// no longer has to already exist on the host. Rejects parent if it falls
// outside the configured browse root (empty by default, see SetBrowseRoot).
func (s *Service) CreateFolder(parent, name string) (Entry, error) {
	if err := WithinRoot(parent, s.browseRoot); err != nil {
		return Entry{}, err
	}
	return CreateFolder(parent, name)
}

// ListLibraries returns every library.
func (s *Service) ListLibraries(ctx context.Context) ([]Library, error) {
	return s.store.ListLibraries(ctx)
}

// DeleteLibrary removes a library and everything imported into it,
// optionally deleting its files from disk too (AC-10/AC-11).
func (s *Service) DeleteLibrary(ctx context.Context, id int64, deleteFiles bool) error {
	return s.store.DeleteLibrary(ctx, id, deleteFiles)
}

// BuildPlan reads tags from every recognized audio file under sourceDir and
// groups them into a per-album plan, computed against libraryID's root.
// Rejects sourceDir if it falls outside the configured browse root (empty by
// default, see SetBrowseRoot) — the browse UI only ever offers paths inside
// it, but sourceDir arrives from the client on every call, so this can't be
// trusted to have been picked through Browse. Also rejects sourceDir if it's
// the same as, or nested inside, any existing library's root (AC-8) —
// importing a library into itself.
func (s *Service) BuildPlan(ctx context.Context, libraryID int64, sourceDir string) (organize.Plan, error) {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return organize.Plan{}, err
	}

	clean := filepath.Clean(sourceDir)
	if err := WithinRoot(clean, s.browseRoot); err != nil {
		return organize.Plan{}, err
	}

	libs, err := s.store.ListLibraries(ctx)
	if err != nil {
		return organize.Plan{}, err
	}
	if sourceOverlapsAnyLibrary(clean, libs) {
		return organize.Plan{}, ErrSourceInsideLibrary
	}

	return organize.BuildPlan(lib.RootPath, clean)
}

// BuildPlanFromStaged is BuildPlan without the source/library overlap check
// — for a directory this process itself staged (an upload's temp
// directory), which can't already be inside a library's root.
func (s *Service) BuildPlanFromStaged(ctx context.Context, libraryID int64, stagedDir string) (organize.Plan, error) {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return organize.Plan{}, err
	}
	return organize.BuildPlan(lib.RootPath, stagedDir)
}

// ValidatePlan recomputes every track's destination and conflict status
// against libraryID's root and the plan's current (possibly reviewer-edited)
// field values, mutating plan in place — see organize.Validate.
func (s *Service) ValidatePlan(ctx context.Context, libraryID int64, plan *organize.Plan) ([]organize.ValidationError, error) {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return organize.Validate(lib.RootPath, plan), nil
}

// sourceOverlapsAnyLibrary reports whether sourceDir is the same as, or
// nested inside, any of libs' root paths.
func sourceOverlapsAnyLibrary(sourceDir string, libs []Library) bool {
	for _, lib := range libs {
		root := filepath.Clean(lib.RootPath)
		if sourceDir == root || strings.HasPrefix(sourceDir, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ConfirmImport validates plan one last time against libraryID's root
// (never trusting a client-sent plan as already valid — see
// organize.Validate's doc comment) and, if it's ready, records a new import
// and starts copying it in the background, returning as soon as it's
// registered — callers must not wait for the copy to finish. errs is
// non-nil (and imp is zero) if the plan still isn't ready; the import is
// only created once no ValidationError remains.
func (s *Service) ConfirmImport(ctx context.Context, libraryID int64, sourceDescription string, plan organize.Plan) (imp Import, errs []organize.ValidationError, err error) {
	errs, err = s.ValidatePlan(ctx, libraryID, &plan)
	if err != nil {
		return Import{}, nil, err
	}
	if len(errs) > 0 {
		return Import{}, errs, nil
	}

	imp, err = s.store.CreateImport(ctx, libraryID, sourceDescription)
	if err != nil {
		return Import{}, nil, err
	}

	go func() {
		summary := s.copier.Copy(context.Background(), s.store, imp.ID, plan)
		if summary.FilesProcessed == 0 && summary.FilesFailed > 0 {
			if err := s.store.MarkImportFailed(context.Background(), imp.ID, "every file failed to copy"); err != nil {
				log.Printf("library: import %d: marking failed: %v", imp.ID, err)
			}
		} else if err := s.store.MarkImportComplete(context.Background(), imp.ID); err != nil {
			log.Printf("library: import %d: marking complete: %v", imp.ID, err)
		}

		// Copy has read everything it needs from plan's sources by now —
		// safe to clean up any staged-upload directory among them (AC-13
		// for uploaded imports; see cleanupStagedSources).
		cleanupStagedSources(plan)

		s.wakeEnricher()
	}()

	return imp, nil, nil
}

// GetImport fetches a single import's current progress.
func (s *Service) GetImport(ctx context.Context, id int64) (Import, error) {
	return s.store.GetImport(ctx, id)
}

// ListImports returns every recorded import, newest first.
func (s *Service) ListImports(ctx context.Context) ([]Import, error) {
	return s.store.ListImports(ctx)
}

// DeleteArtist removes an artist and everything attributed to them,
// optionally deleting their files from disk too (AC-13). Gallery rows
// (artist_photos) cascade with the artist row, but the image files
// themselves live outside the DB, so they're cleaned up here too — same
// deleteFiles gate as track files, since orphaned artwork is exactly the
// on-disk-leftover case AC-13 exists to avoid.
func (s *Service) DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error {
	if err := s.store.DeleteArtist(ctx, id, deleteFiles); err != nil {
		return err
	}
	if deleteFiles && s.images != nil {
		if err := s.images.DeleteAll("artist", id); err != nil {
			log.Printf("library: artist %d: deleting artwork files: %v", id, err)
		}
	}
	return nil
}

// DeleteAlbum removes an album and its tracks, optionally deleting their
// files from disk too (AC-13). See DeleteArtist for why album_covers'
// image files need their own cleanup alongside the cascaded DB rows.
func (s *Service) DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error {
	if err := s.store.DeleteAlbum(ctx, id, deleteFiles); err != nil {
		return err
	}
	if deleteFiles && s.images != nil {
		if err := s.images.DeleteAll("album", id); err != nil {
			log.Printf("library: album %d: deleting artwork files: %v", id, err)
		}
	}
	return nil
}

// RetryArtistArt resets an artist's art status to pending and wakes the
// background enrichment job immediately (TDR 007) rather than waiting for
// the next import or backend restart — the same fire-and-forget goroutine
// ConfirmImport already launches after a copy completes. Returns once the
// reset itself is recorded; the actual lookup runs asynchronously, so a
// caller wanting the outcome polls GetArtist afterward.
func (s *Service) RetryArtistArt(ctx context.Context, id int64) error {
	if err := s.store.ResetArtistArt(ctx, id); err != nil {
		return err
	}
	s.wakeEnricher()
	return nil
}

// RetryAlbumArt is RetryArtistArt's album counterpart.
func (s *Service) RetryAlbumArt(ctx context.Context, id int64) error {
	if err := s.store.ResetAlbumArt(ctx, id); err != nil {
		return err
	}
	s.wakeEnricher()
	return nil
}

// wakeEnricher launches one enrichment pass in the background, ignoring a
// nil enricher (ARTWORK_DIR unset — see main.go) the same way ConfirmImport
// already does.
func (s *Service) wakeEnricher() {
	if s.enricher == nil {
		return
	}
	go func() {
		sum := s.enricher.Run(context.Background())
		log.Printf("library: enrichment: %d found, %d not found, %d failed", sum.Found, sum.NotFound, sum.Failed)
	}()
}

// ErrArtworkNotConfigured is returned by UploadArtistArt/UploadAlbumArt
// when ARTWORK_DIR isn't set — there's nowhere on disk to save an upload
// to, the same constraint the background enrichment job is already under.
var ErrArtworkNotConfigured = errors.New("artwork storage is not configured")

// UploadArtistArt adds data as a new photo in id's gallery (AC-3: upload
// always adds, never replaces) — bypassing MusicBrainz/Wikidata entirely
// (TDR 007), synchronously (no queueing, unlike retry). The image is
// saved via the shared ImageStore (whose content hash drives AC-5's
// dedup), then art_status is marked Found so the background enrichment
// job treats this artist's art as settled — the empty thumb/full paths in
// that call are deliberate: the gallery entry itself was already added
// above, so SetArtistArt here only needs to flip the status.
func (s *Service) UploadArtistArt(ctx context.Context, id int64, data []byte) (ArtistDetail, error) {
	if s.images == nil {
		return ArtistDetail{}, ErrArtworkNotConfigured
	}
	thumbURL, fullURL, hash, err := s.images.Save("artist", id, data)
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("saving artist photo: %w", err)
	}
	if _, err := s.store.AddArtistPhoto(ctx, id, thumbURL, fullURL, "upload", hash); err != nil {
		return ArtistDetail{}, err
	}
	if err := s.store.SetArtistArt(ctx, id, enrich.Found, "", ""); err != nil {
		return ArtistDetail{}, err
	}
	return s.store.GetArtist(ctx, id)
}

// UploadAlbumArt is UploadArtistArt's album counterpart. The uploaded
// image has no known Cover Art Archive/embedded-tag type, so it's added
// with an empty pictureType.
func (s *Service) UploadAlbumArt(ctx context.Context, id int64, data []byte) (AlbumDetail, error) {
	if s.images == nil {
		return AlbumDetail{}, ErrArtworkNotConfigured
	}
	thumbURL, fullURL, hash, err := s.images.Save("album", id, data)
	if err != nil {
		return AlbumDetail{}, fmt.Errorf("saving album cover: %w", err)
	}
	if _, err := s.store.AddAlbumCover(ctx, id, thumbURL, fullURL, "upload", "", hash); err != nil {
		return AlbumDetail{}, err
	}
	if err := s.store.SetAlbumArt(ctx, id, enrich.Found, "", ""); err != nil {
		return AlbumDetail{}, err
	}
	return s.store.GetAlbum(ctx, id)
}

// ListArtistPhotos returns every photo in an artist's gallery (AC-1).
func (s *Service) ListArtistPhotos(ctx context.Context, artistID int64) ([]ArtistPhoto, error) {
	return s.store.ListArtistPhotos(ctx, artistID)
}

// SetArtistPrimaryPhoto marks photoID as the one shown in list views and
// grid tiles for this artist (AC-2).
func (s *Service) SetArtistPrimaryPhoto(ctx context.Context, artistID, photoID int64) error {
	return s.store.SetArtistPrimaryPhoto(ctx, artistID, photoID)
}

// DeleteArtistPhoto removes a photo from an artist's gallery (AC-4),
// optionally also deleting its files from disk — the same explicit
// keep-vs-delete-file choice DeleteArtist/DeleteAlbum already offer for
// tracks. A missing/misconfigured ImageStore just skips the file removal;
// the catalog reference is still gone either way.
func (s *Service) DeleteArtistPhoto(ctx context.Context, artistID, photoID int64, deleteFile bool) error {
	thumbURL, fullURL, err := s.store.DeleteArtistPhoto(ctx, artistID, photoID)
	if err != nil {
		return err
	}
	if deleteFile && s.images != nil {
		if err := s.images.Delete(thumbURL, fullURL); err != nil {
			log.Printf("library: artist %d: deleting photo file: %v", artistID, err)
		}
	}
	return nil
}

// ListAlbumCovers returns every cover in an album's gallery (AC-1).
func (s *Service) ListAlbumCovers(ctx context.Context, albumID int64) ([]AlbumCover, error) {
	return s.store.ListAlbumCovers(ctx, albumID)
}

// SetAlbumPrimaryCover marks coverID as the one shown in list views and
// grid tiles for this album (AC-2).
func (s *Service) SetAlbumPrimaryCover(ctx context.Context, albumID, coverID int64) error {
	return s.store.SetAlbumPrimaryCover(ctx, albumID, coverID)
}

// DeleteAlbumCover is DeleteArtistPhoto's album counterpart.
func (s *Service) DeleteAlbumCover(ctx context.Context, albumID, coverID int64, deleteFile bool) error {
	thumbURL, fullURL, err := s.store.DeleteAlbumCover(ctx, albumID, coverID)
	if err != nil {
		return err
	}
	if deleteFile && s.images != nil {
		if err := s.images.Delete(thumbURL, fullURL); err != nil {
			log.Printf("library: album %d: deleting cover file: %v", albumID, err)
		}
	}
	return nil
}

// ErrMetadataSearchNotConfigured is returned by SearchArtists and its
// sibling passthroughs when SetMusicBrainzSearch was never called.
var ErrMetadataSearchNotConfigured = errors.New("metadata search is not configured")

// SearchArtists looks up every MusicBrainz artist matching name, for the
// review screen's "Look up metadata" flow (TDR 012) to pick from.
func (s *Service) SearchArtists(ctx context.Context, name string) ([]enrich.ArtistMatch, error) {
	if s.musicBrainz == nil {
		return nil, ErrMetadataSearchNotConfigured
	}
	return s.musicBrainz.SearchArtists(ctx, name)
}

// ArtistReleaseGroups lists every album MusicBrainz has on record for the
// artist identified by artistMBID.
func (s *Service) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]enrich.ReleaseGroupMatch, error) {
	if s.musicBrainz == nil {
		return nil, ErrMetadataSearchNotConfigured
	}
	return s.musicBrainz.ArtistReleaseGroups(ctx, artistMBID)
}

// ReleaseGroupReleases lists every specific release/edition under the
// release-group identified by releaseGroupMBID.
func (s *Service) ReleaseGroupReleases(ctx context.Context, releaseGroupMBID string) ([]enrich.ReleaseMatch, error) {
	if s.musicBrainz == nil {
		return nil, ErrMetadataSearchNotConfigured
	}
	return s.musicBrainz.ReleaseGroupReleases(ctx, releaseGroupMBID)
}

// ReleaseTracks fetches the full track listing for the release identified
// by releaseMBID.
func (s *Service) ReleaseTracks(ctx context.Context, releaseMBID string) ([]enrich.Track, error) {
	if s.musicBrainz == nil {
		return nil, ErrMetadataSearchNotConfigured
	}
	return s.musicBrainz.ReleaseTracks(ctx, releaseMBID)
}

// ListArtists returns a page of artists matching opts.
func (s *Service) ListArtists(ctx context.Context, opts ListOptions) (Page[Artist], error) {
	return s.store.ListArtists(ctx, normalizeListOptions(opts))
}

// GetArtist fetches a single artist by ID, with their albums.
func (s *Service) GetArtist(ctx context.Context, id int64) (ArtistDetail, error) {
	return s.store.GetArtist(ctx, id)
}

// ListAlbums returns a page of albums matching opts.
func (s *Service) ListAlbums(ctx context.Context, opts ListOptions) (Page[Album], error) {
	return s.store.ListAlbums(ctx, normalizeListOptions(opts))
}

// GetAlbum fetches a single album by ID, with its track listing.
func (s *Service) GetAlbum(ctx context.Context, id int64) (AlbumDetail, error) {
	return s.store.GetAlbum(ctx, id)
}

// ListSongs returns a page of songs (tracks) matching opts.
func (s *Service) ListSongs(ctx context.Context, opts ListOptions) (Page[Song], error) {
	return s.store.ListSongs(ctx, normalizeListOptions(opts))
}

// GetSongPath resolves id to its on-disk path (TDR 015), for the audio
// streaming handler — never itself part of a List/Get JSON response.
func (s *Service) GetSongPath(ctx context.Context, id int64) (string, error) {
	return s.store.GetSongPath(ctx, id)
}
