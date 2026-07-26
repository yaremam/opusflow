package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// fakeImportStore is an in-memory ImportStore for testing Service's
// orchestration without a real database.
type fakeImportStore struct {
	mu             sync.Mutex
	nextID         int64
	imports        map[int64]Import
	tracks         []organize.CopiedTrack
	errors         []string
	deletedArtists []int64
	deletedAlbums  []int64

	nextLibID   int64
	libraries   map[int64]Library
	deletedLibs []int64
}

func newFakeImportStore() *fakeImportStore {
	return &fakeImportStore{imports: make(map[int64]Import), libraries: make(map[int64]Library)}
}

// seedLibrary creates a library directly in the fake store (no directory
// validation, unlike the real Store — that's covered by libraries_test.go
// against real Postgres), returning its ID for tests that just need a
// valid libraryID to build/confirm a plan against.
func (f *fakeImportStore) seedLibrary(rootPath string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextLibID++
	f.libraries[f.nextLibID] = Library{ID: f.nextLibID, Name: "Test Library", RootPath: rootPath, CreatedAt: time.Now()}
	return f.nextLibID
}

func (f *fakeImportStore) CreateLibrary(_ context.Context, name, rootPath string) (Library, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextLibID++
	lib := Library{ID: f.nextLibID, Name: name, RootPath: rootPath, CreatedAt: time.Now()}
	f.libraries[lib.ID] = lib
	return lib, nil
}

func (f *fakeImportStore) ListLibraries(_ context.Context) ([]Library, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	libs := make([]Library, 0, len(f.libraries))
	for id := int64(1); id <= f.nextLibID; id++ {
		if lib, ok := f.libraries[id]; ok {
			libs = append(libs, lib)
		}
	}
	return libs, nil
}

func (f *fakeImportStore) GetLibrary(_ context.Context, id int64) (Library, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lib, ok := f.libraries[id]
	if !ok {
		return Library{}, ErrLibraryNotFound
	}
	return lib, nil
}

func (f *fakeImportStore) DeleteLibrary(_ context.Context, id int64, deleteFiles bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedLibs = append(f.deletedLibs, id)
	return nil
}

func (f *fakeImportStore) CreateImport(_ context.Context, libraryID int64, sourceDescription string) (Import, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	imp := Import{ID: f.nextID, LibraryID: libraryID, SourceDescription: sourceDescription, Status: ImportStatusCopying, FileErrors: []FileError{}, CreatedAt: time.Now()}
	f.imports[imp.ID] = imp
	return imp, nil
}

func (f *fakeImportStore) GetImport(_ context.Context, id int64) (Import, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	imp, ok := f.imports[id]
	if !ok {
		return Import{}, ErrImportNotFound
	}
	return imp, nil
}

func (f *fakeImportStore) ListImports(_ context.Context) ([]Import, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	imports := make([]Import, 0, len(f.imports))
	for id := int64(1); id <= f.nextID; id++ {
		if imp, ok := f.imports[id]; ok {
			imports = append(imports, imp)
		}
	}
	return imports, nil
}

func (f *fakeImportStore) MarkImportComplete(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	imp := f.imports[id]
	imp.Status = ImportStatusComplete
	f.imports[id] = imp
	return nil
}

func (f *fakeImportStore) MarkImportFailed(_ context.Context, id int64, errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	imp := f.imports[id]
	imp.Status = ImportStatusFailed
	imp.Error = errMsg
	f.imports[id] = imp
	return nil
}

func (f *fakeImportStore) InsertTrack(_ context.Context, t organize.CopiedTrack) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tracks = append(f.tracks, t)
	return nil
}

func (f *fakeImportStore) RecordImportError(_ context.Context, importID int64, path, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors = append(f.errors, path+": "+message)
	return nil
}

func (f *fakeImportStore) UpdateImportProgress(_ context.Context, id int64, processed, total int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	imp := f.imports[id]
	imp.FilesProcessed = processed
	imp.FilesTotal = total
	f.imports[id] = imp
	return nil
}

func (f *fakeImportStore) DeleteArtist(_ context.Context, id int64, deleteFiles bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedArtists = append(f.deletedArtists, id)
	return nil
}

func (f *fakeImportStore) DeleteAlbum(_ context.Context, id int64, deleteFiles bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletedAlbums = append(f.deletedAlbums, id)
	return nil
}

// Catalog methods aren't exercised by import-orchestration tests — stubbed
// just to satisfy ImportStore. catalogCapturingStore (service_catalog_test.go)
// overrides these with real recording behavior for tests that do exercise them.
func (f *fakeImportStore) ListArtists(context.Context, ListOptions) (Page[Artist], error) {
	return Page[Artist]{Items: []Artist{}}, nil
}
func (f *fakeImportStore) GetArtist(context.Context, int64) (ArtistDetail, error) {
	return ArtistDetail{}, ErrArtistNotFound
}
func (f *fakeImportStore) ListAlbums(context.Context, ListOptions) (Page[Album], error) {
	return Page[Album]{Items: []Album{}}, nil
}
func (f *fakeImportStore) GetAlbum(context.Context, int64) (AlbumDetail, error) {
	return AlbumDetail{}, ErrAlbumNotFound
}
func (f *fakeImportStore) ListSongs(context.Context, ListOptions) (Page[Song], error) {
	return Page[Song]{Items: []Song{}}, nil
}

type copyCall struct {
	importID int64
	plan     organize.Plan
}

// recordingCopier is a non-blocking fake Copier: it records the call and
// signals doneCh so tests can deterministically wait for the goroutine
// ConfirmImport spawns, without sleeping.
type recordingCopier struct {
	mu      sync.Mutex
	calls   []copyCall
	doneCh  chan struct{}
	summary organize.RunSummary
}

func newRecordingCopier() *recordingCopier {
	return &recordingCopier{doneCh: make(chan struct{}, 10), summary: organize.RunSummary{FilesProcessed: 1}}
}

func (r *recordingCopier) Copy(_ context.Context, _ organize.Store, importID int64, plan organize.Plan) organize.RunSummary {
	r.mu.Lock()
	r.calls = append(r.calls, copyCall{importID, plan})
	r.mu.Unlock()
	r.doneCh <- struct{}{}
	return r.summary
}

func (r *recordingCopier) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-r.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Copy to be called")
	}
}

// blockingCopier never returns from Copy until the test closes block, used
// to prove ConfirmImport doesn't wait for the copy to finish.
type blockingCopier struct{ block chan struct{} }

func (b *blockingCopier) Copy(_ context.Context, _ organize.Store, _ int64, _ organize.Plan) organize.RunSummary {
	<-b.block
	return organize.RunSummary{}
}

// recordingEnricher is a non-blocking fake Enricher: it records the call
// and signals doneCh, the same pattern recordingCopier uses.
type recordingEnricher struct {
	mu     sync.Mutex
	calls  int
	doneCh chan struct{}
}

func newRecordingEnricher() *recordingEnricher {
	return &recordingEnricher{doneCh: make(chan struct{}, 10)}
}

func (r *recordingEnricher) Run(context.Context) enrich.RunSummary {
	r.mu.Lock()
	r.calls++
	r.mu.Unlock()
	r.doneCh <- struct{}{}
	return enrich.RunSummary{}
}

func (r *recordingEnricher) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-r.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to be called")
	}
}

func validPlan() organize.Plan {
	return organize.Plan{Albums: []organize.Album{{
		Artist: "Artist", Album: "Album", Year: 2000,
		Tracks: []organize.Track{{SourcePath: "/src/one.mp3", TrackNumber: 1, Title: "Title"}},
	}}}
}

func TestConfirmImportReturnsBeforeCopyCompletes(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := &blockingCopier{block: make(chan struct{})}
	svc := NewService(store, copier)

	done := make(chan error, 1)
	go func() {
		_, _, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan())
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ConfirmImport: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ConfirmImport did not return promptly — looks like it's waiting on the copy")
	}

	close(copier.block) // let the background goroutine finish, avoid leaking it
}

func TestConfirmImportSuccess(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	svc := NewService(store, copier)

	imp, errs, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan())
	if err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("errs = %+v, want none", errs)
	}
	if imp.SourceDescription != "/music/src" {
		t.Fatalf("SourceDescription = %q", imp.SourceDescription)
	}
	if imp.LibraryID != libID {
		t.Fatalf("LibraryID = %d, want %d", imp.LibraryID, libID)
	}

	copier.waitForCall(t)
	if len(copier.calls) != 1 || copier.calls[0].importID != imp.ID {
		t.Fatalf("copier.calls = %+v, want one call for import %d", copier.calls, imp.ID)
	}
}

func TestConfirmImportRejectsInvalidPlan(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	svc := NewService(store, copier)

	incomplete := organize.Plan{Albums: []organize.Album{{
		Artist: "", Album: "Album", Year: 2000,
		Tracks: []organize.Track{{SourcePath: "/src/one.mp3"}},
	}}}

	imp, errs, err := svc.ConfirmImport(ctx(), libID, "/music/src", incomplete)
	if err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected validation errors for an incomplete plan")
	}
	if imp.ID != 0 {
		t.Fatalf("expected no import to be created, got %+v", imp)
	}
	if len(copier.calls) != 0 {
		t.Fatalf("expected no copy to be started, got %+v", copier.calls)
	}
}

func TestConfirmImportRejectsUnknownLibrary(t *testing.T) {
	store := newFakeImportStore()
	copier := newRecordingCopier()
	svc := NewService(store, copier)

	_, _, err := svc.ConfirmImport(ctx(), 999999, "/music/src", validPlan())
	if !errors.Is(err, ErrLibraryNotFound) {
		t.Fatalf("ConfirmImport error = %v, want ErrLibraryNotFound", err)
	}
}

func TestConfirmImportMarksCompleteAfterCopySucceeds(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	copier.summary = organize.RunSummary{FilesProcessed: 1, FilesFailed: 1}
	svc := NewService(store, copier)

	imp, _, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan())
	if err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}
	copier.waitForCall(t)

	waitForImportStatus(t, svc, imp.ID, ImportStatusComplete)
}

func TestConfirmImportMarksFailedWhenEveryFileFails(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	copier.summary = organize.RunSummary{FilesProcessed: 0, FilesFailed: 1}
	svc := NewService(store, copier)

	imp, _, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan())
	if err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}
	copier.waitForCall(t)

	waitForImportStatus(t, svc, imp.ID, ImportStatusFailed)
}

func waitForImportStatus(t *testing.T, svc *Service, id int64, want ImportStatus) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		imp, err := svc.GetImport(ctx(), id)
		if err != nil {
			t.Fatalf("GetImport: %v", err)
		}
		if imp.Status == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("Status = %q after timeout, want %q", imp.Status, want)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestConfirmImportRunsEnrichmentAfterCopyCompletes(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	enricher := newRecordingEnricher()
	svc := NewService(store, copier)
	svc.SetEnricher(enricher)

	if _, _, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan()); err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}

	enricher.waitForCall(t)
	if enricher.calls != 1 {
		t.Fatalf("enricher.calls = %d, want 1", enricher.calls)
	}
}

func TestConfirmImportWithoutEnricherDoesNotPanic(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	copier := newRecordingCopier()
	svc := NewService(store, copier)

	if _, _, err := svc.ConfirmImport(ctx(), libID, "/music/src", validPlan()); err != nil {
		t.Fatalf("ConfirmImport: %v", err)
	}
	copier.waitForCall(t)
}

func TestBuildPlanRejectsSourceInsideLibrary(t *testing.T) {
	store := newFakeImportStore()
	libRoot := t.TempDir()
	store.seedLibrary(libRoot)
	otherLibID := store.seedLibrary(t.TempDir())
	svc := NewService(store, newRecordingCopier())

	sourceInsideLibRoot := filepath.Join(libRoot, "some-download")
	if err := os.MkdirAll(sourceInsideLibRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := svc.BuildPlan(ctx(), otherLibID, sourceInsideLibRoot)
	if !errors.Is(err, ErrSourceInsideLibrary) {
		t.Fatalf("BuildPlan error = %v, want ErrSourceInsideLibrary", err)
	}
}

func TestBuildPlanReadsTagsFromSourceDirectory(t *testing.T) {
	store := newFakeImportStore()
	libID := store.seedLibrary(t.TempDir())
	sub := t.TempDir()
	svc := NewService(store, newRecordingCopier())

	plan, err := svc.BuildPlan(ctx(), libID, sub)
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Albums) != 0 {
		t.Fatalf("plan = %+v, want no albums for an empty directory", plan)
	}
}

func TestBuildPlanRejectsUnknownLibrary(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	_, err := svc.BuildPlan(ctx(), 999999, t.TempDir())
	if !errors.Is(err, ErrLibraryNotFound) {
		t.Fatalf("BuildPlan error = %v, want ErrLibraryNotFound", err)
	}
}

func TestDeleteArtistDelegatesToStore(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.DeleteArtist(ctx(), 42, true); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}
	if len(store.deletedArtists) != 1 || store.deletedArtists[0] != 42 {
		t.Fatalf("deletedArtists = %+v, want [42]", store.deletedArtists)
	}
}

func TestDeleteAlbumDelegatesToStore(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.DeleteAlbum(ctx(), 7, false); err != nil {
		t.Fatalf("DeleteAlbum: %v", err)
	}
	if len(store.deletedAlbums) != 1 || store.deletedAlbums[0] != 7 {
		t.Fatalf("deletedAlbums = %+v, want [7]", store.deletedAlbums)
	}
}

func TestDeleteLibraryDelegatesToStore(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())

	if err := svc.DeleteLibrary(ctx(), 3, true); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}
	if len(store.deletedLibs) != 1 || store.deletedLibs[0] != 3 {
		t.Fatalf("deletedLibs = %+v, want [3]", store.deletedLibs)
	}
}

func TestCreateLibraryDelegatesToStore(t *testing.T) {
	store := newFakeImportStore()
	svc := NewService(store, newRecordingCopier())
	root := t.TempDir()

	lib, err := svc.CreateLibrary(ctx(), "Main Collection", root)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	if lib.Name != "Main Collection" || lib.RootPath != root {
		t.Fatalf("lib = %+v", lib)
	}
}

func TestListLibrariesDelegatesToStore(t *testing.T) {
	store := newFakeImportStore()
	store.seedLibrary(t.TempDir())
	store.seedLibrary(t.TempDir())
	svc := NewService(store, newRecordingCopier())

	libs, err := svc.ListLibraries(ctx())
	if err != nil {
		t.Fatalf("ListLibraries: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("libs = %+v, want 2", libs)
	}
}
