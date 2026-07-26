package library

import (
	"context"
	"log"
	"path/filepath"

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

// ImportStore is the persistence Service needs — import management plus
// browsing the artist/album/song catalog those imports populate, plus
// organize.Store's methods so it can be handed straight to a Copier.
// *Store satisfies this as its one production adapter; tests substitute an
// in-memory fake so orchestration logic (validation, copy-goroutine timing,
// list-options normalization) can be tested without a database.
type ImportStore interface {
	organize.Store

	CreateImport(ctx context.Context, sourceDescription string) (Import, error)
	GetImport(ctx context.Context, id int64) (Import, error)
	ListImports(ctx context.Context) ([]Import, error)
	MarkImportComplete(ctx context.Context, id int64) error
	MarkImportFailed(ctx context.Context, id int64, errMsg string) error

	DeleteArtist(ctx context.Context, id int64, deleteFiles bool) error
	DeleteAlbum(ctx context.Context, id int64, deleteFiles bool) error

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

// Service orchestrates organize-on-import: browsing configured source
// roots, building and validating a plan against LIBRARY_ROOT, confirming it
// (which copies files in the background), and removing catalog entries.
type Service struct {
	sourceRoots Roots
	libraryRoot string
	store       ImportStore
	copier      Copier
	enricher    Enricher
}

// NewService builds a Service. sourceRoots is where "browse a server
// folder" is allowed to look (IMPORT_SOURCE_ROOTS); libraryRoot is where
// confirmed imports get written (LIBRARY_ROOT).
func NewService(sourceRoots Roots, libraryRoot string, store ImportStore, copier Copier) *Service {
	return &Service{sourceRoots: sourceRoots, libraryRoot: libraryRoot, store: store, copier: copier}
}

// SetEnricher wires up the background artwork/info job, run after every
// import's copy completes. Kept as a setter rather than a NewService
// parameter so existing callers (tests included) that don't need
// enrichment aren't forced to construct one; nil (the default) just means
// ConfirmImport's copy-completion hook is a no-op.
func (s *Service) SetEnricher(enricher Enricher) {
	s.enricher = enricher
}

// ListSourceRoots returns the configured import source roots.
func (s *Service) ListSourceRoots() Roots {
	return s.sourceRoots
}

// Browse lists the immediate subdirectories of path.
func (s *Service) Browse(path string) ([]Entry, error) {
	return s.sourceRoots.Browse(path)
}

// BuildPlan reads tags from every recognized audio file under sourceDir and
// groups them into a per-album plan, computed against LIBRARY_ROOT.
// sourceDir must be nested under one of the configured IMPORT_SOURCE_ROOTS.
func (s *Service) BuildPlan(sourceDir string) (organize.Plan, error) {
	clean := filepath.Clean(sourceDir)
	if _, err := s.sourceRoots.ValidateDirectory(clean); err != nil {
		return organize.Plan{}, err
	}
	return organize.BuildPlan(s.libraryRoot, clean)
}

// BuildPlanFromStaged is BuildPlan without the IMPORT_SOURCE_ROOTS check —
// for a directory this process itself staged (an upload's temp directory),
// not one a client asked to browse into.
func (s *Service) BuildPlanFromStaged(stagedDir string) (organize.Plan, error) {
	return organize.BuildPlan(s.libraryRoot, stagedDir)
}

// ValidatePlan recomputes every track's destination and conflict status
// against the plan's current (possibly reviewer-edited) field values,
// mutating plan in place — see organize.Validate.
func (s *Service) ValidatePlan(plan *organize.Plan) []organize.ValidationError {
	return organize.Validate(s.libraryRoot, plan)
}

// ConfirmImport validates plan one last time (never trusting a client-sent
// plan as already valid — see organize.Validate's doc comment) and, if it's
// ready, records a new import and starts copying it in the background,
// returning as soon as it's registered — callers must not wait for the copy
// to finish. errs is non-nil (and imp is zero) if the plan still isn't
// ready; the import is only created once no ValidationError remains.
func (s *Service) ConfirmImport(ctx context.Context, sourceDescription string, plan organize.Plan) (imp Import, errs []organize.ValidationError, err error) {
	if errs := s.ValidatePlan(&plan); len(errs) > 0 {
		return Import{}, errs, nil
	}

	imp, err = s.store.CreateImport(ctx, sourceDescription)
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

		if s.enricher != nil {
			sum := s.enricher.Run(context.Background())
			log.Printf("library: enrichment: %d found, %d not found, %d failed", sum.Found, sum.NotFound, sum.Failed)
		}
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
