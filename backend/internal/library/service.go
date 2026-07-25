package library

import (
	"context"
	"path/filepath"
	"sync"
)

// Scanner starts an asynchronous scan of a registered directory. The real
// implementation is scan.Scanner; tests substitute a fake.
type Scanner interface {
	Scan(ctx context.Context, directoryID int64, path string)
}

// DirectoryStore is the persistence Service needs — directory management
// plus browsing the artist/album/song catalog those directories' scans
// populate — the subset of Store's methods it actually calls. *Store
// satisfies this as its one production adapter; tests substitute an
// in-memory fake so orchestration logic (validation ordering, scan-goroutine
// timing, cancellation, list-options normalization) can be tested without a
// database.
type DirectoryStore interface {
	AddDirectory(ctx context.Context, root, path string) (Directory, error)
	GetDirectory(ctx context.Context, id int64) (Directory, error)
	ListDirectories(ctx context.Context) ([]Directory, error)
	RemoveDirectory(ctx context.Context, id int64) error

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

// Service orchestrates library directory management: browsing configured
// roots, registering and scanning new directories, and removing them.
type Service struct {
	roots   Roots
	store   DirectoryStore
	scanner Scanner

	mu          sync.Mutex
	activeScans map[int64]context.CancelFunc
}

// NewService builds a Service.
func NewService(roots Roots, store DirectoryStore, scanner Scanner) *Service {
	return &Service{roots: roots, store: store, scanner: scanner, activeScans: make(map[int64]context.CancelFunc)}
}

// ListRoots returns the configured library roots.
func (s *Service) ListRoots() Roots {
	return s.roots
}

// Browse lists the immediate subdirectories of path.
func (s *Service) Browse(path string) ([]Entry, error) {
	return s.roots.Browse(path)
}

// ListDirectories returns every registered directory.
func (s *Service) ListDirectories(ctx context.Context) ([]Directory, error) {
	return s.store.ListDirectories(ctx)
}

// GetDirectory fetches a single registered directory.
func (s *Service) GetDirectory(ctx context.Context, id int64) (Directory, error) {
	return s.store.GetDirectory(ctx, id)
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

// AddDirectory registers path as a new library directory and starts
// scanning it in the background, returning as soon as it's registered —
// callers must not wait for the scan to finish (AC-3). The scan runs under
// its own cancellable context, independent of the request's, so it keeps
// going after this call returns — but RemoveDirectory can still cancel it
// if the directory is removed before the scan finishes.
func (s *Service) AddDirectory(ctx context.Context, path string) (Directory, error) {
	clean := filepath.Clean(path)

	root, err := s.roots.ValidateDirectory(clean)
	if err != nil {
		return Directory{}, err
	}

	dir, err := s.store.AddDirectory(ctx, root, clean)
	if err != nil {
		return Directory{}, err
	}

	scanCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.activeScans[dir.ID] = cancel
	s.mu.Unlock()

	go func() {
		defer s.endScan(dir.ID)
		s.scanner.Scan(scanCtx, dir.ID, dir.Path)
	}()

	return dir, nil
}

// endScan drops directoryID's cancel func once its scan goroutine returns,
// whether it finished normally or was cancelled.
func (s *Service) endScan(directoryID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.activeScans, directoryID)
}

// RemoveDirectory deletes a registered directory and, via cascade, its
// tracks and file errors. If a scan for this directory is still running, it
// cancels that scan first so it stops touching a directory that's about to
// stop existing, rather than continuing to walk the filesystem and write to
// rows that are already gone.
func (s *Service) RemoveDirectory(ctx context.Context, id int64) error {
	s.mu.Lock()
	if cancel, ok := s.activeScans[id]; ok {
		cancel()
	}
	s.mu.Unlock()

	return s.store.RemoveDirectory(ctx, id)
}
