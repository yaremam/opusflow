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

// ImportStore is the persistence Service needs — library and import
// management plus browsing the artist/album/song catalog those imports
// populate, plus organize.Store's methods so it can be handed straight to a
// Copier. *Store satisfies this as its one production adapter; tests
// substitute an in-memory fake so orchestration logic (validation,
// copy-goroutine timing, list-options normalization) can be tested without
// a database.
type ImportStore interface {
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

	DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error
	DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error

	ResetArtistArt(ctx context.Context, id int64) error
	ResetAlbumArt(ctx context.Context, id int64) error
	SetArtistArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error
	SetAlbumArt(ctx context.Context, id int64, status enrich.Status, thumbPath, fullPath string) error

	ListArtists(ctx context.Context, opts ListOptions) (Page[Artist], error)
	GetArtist(ctx context.Context, id int64) (ArtistDetail, error)
	ListAlbums(ctx context.Context, opts ListOptions) (Page[Album], error)
	GetAlbum(ctx context.Context, id int64) (AlbumDetail, error)
	ListSongs(ctx context.Context, opts ListOptions) (Page[Song], error)
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
	store      ImportStore
	copier     Copier
	enricher   Enricher
	browseRoot string
	images     *enrich.ImageStore
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
// optionally deleting their files from disk too (AC-13).
func (s *Service) DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error {
	return s.store.DeleteArtist(ctx, id, deleteFiles)
}

// DeleteAlbum removes an album and its tracks, optionally deleting their
// files from disk too (AC-13).
func (s *Service) DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error {
	return s.store.DeleteAlbum(ctx, id, deleteFiles)
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

// UploadArtistArt saves data as id's photo — bypassing MusicBrainz/Cover
// Art Archive entirely (TDR 007), synchronously (no queueing, unlike
// retry): the same thumb/full variants the automatic lookup produces are
// written via the shared ImageStore, then recorded as Found immediately.
func (s *Service) UploadArtistArt(ctx context.Context, id int64, data []byte) (Artist, error) {
	if s.images == nil {
		return Artist{}, ErrArtworkNotConfigured
	}
	thumbURL, fullURL, err := s.images.Save("artist", id, data)
	if err != nil {
		return Artist{}, fmt.Errorf("saving artist photo: %w", err)
	}
	if err := s.store.SetArtistArt(ctx, id, enrich.Found, thumbURL, fullURL); err != nil {
		return Artist{}, err
	}
	detail, err := s.store.GetArtist(ctx, id)
	if err != nil {
		return Artist{}, err
	}
	return detail.Artist, nil
}

// UploadAlbumArt is UploadArtistArt's album counterpart.
func (s *Service) UploadAlbumArt(ctx context.Context, id int64, data []byte) (Album, error) {
	if s.images == nil {
		return Album{}, ErrArtworkNotConfigured
	}
	thumbURL, fullURL, err := s.images.Save("album", id, data)
	if err != nil {
		return Album{}, fmt.Errorf("saving album cover: %w", err)
	}
	if err := s.store.SetAlbumArt(ctx, id, enrich.Found, thumbURL, fullURL); err != nil {
		return Album{}, err
	}
	detail, err := s.store.GetAlbum(ctx, id)
	if err != nil {
		return Album{}, err
	}
	return detail.Album, nil
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
