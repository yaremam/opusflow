package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeDirectoryStore is an in-memory DirectoryStore for testing Service's
// orchestration without a real database — mirrors fakeProgressStore in
// scan/scanner_test.go.
type fakeDirectoryStore struct {
	mu     sync.Mutex
	nextID int64
	dirs   map[int64]Directory
	byPath map[string]int64
}

func newFakeDirectoryStore() *fakeDirectoryStore {
	return &fakeDirectoryStore{dirs: make(map[int64]Directory), byPath: make(map[string]int64)}
}

func (f *fakeDirectoryStore) AddDirectory(_ context.Context, root, path string) (Directory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byPath[path]; exists {
		return Directory{}, ErrDuplicateDirectory
	}
	f.nextID++
	dir := Directory{
		ID:         f.nextID,
		Root:       root,
		Path:       path,
		Status:     StatusScanning,
		FileErrors: []FileError{},
		CreatedAt:  time.Now(),
	}
	f.dirs[dir.ID] = dir
	f.byPath[path] = dir.ID
	return dir, nil
}

func (f *fakeDirectoryStore) GetDirectory(_ context.Context, id int64) (Directory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dir, ok := f.dirs[id]
	if !ok {
		return Directory{}, ErrDirectoryNotFound
	}
	return dir, nil
}

func (f *fakeDirectoryStore) ListDirectories(_ context.Context) ([]Directory, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	dirs := make([]Directory, 0, len(f.dirs))
	for id := int64(1); id <= f.nextID; id++ {
		if dir, ok := f.dirs[id]; ok {
			dirs = append(dirs, dir)
		}
	}
	return dirs, nil
}

func (f *fakeDirectoryStore) RemoveDirectory(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	dir, ok := f.dirs[id]
	if !ok {
		return ErrDirectoryNotFound
	}
	delete(f.dirs, id)
	delete(f.byPath, dir.Path)
	return nil
}

type scanCall struct {
	directoryID int64
	path        string
}

// recordingScanner is a non-blocking fake Scanner: it records the call and
// signals doneCh so tests can deterministically wait for the goroutine
// AddDirectory spawns, without sleeping.
type recordingScanner struct {
	mu     sync.Mutex
	calls  []scanCall
	doneCh chan struct{}
}

func newRecordingScanner() *recordingScanner {
	return &recordingScanner{doneCh: make(chan struct{}, 10)}
}

func (r *recordingScanner) Scan(_ context.Context, directoryID int64, path string) {
	r.mu.Lock()
	r.calls = append(r.calls, scanCall{directoryID, path})
	r.mu.Unlock()
	r.doneCh <- struct{}{}
}

func (r *recordingScanner) waitForCall(t *testing.T) {
	t.Helper()
	select {
	case <-r.doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Scan to be called")
	}
}

// blockingScanner never returns from Scan until the test closes block,
// used to prove AddDirectory doesn't wait for the scan to finish (AC-3).
type blockingScanner struct{ block chan struct{} }

func (b *blockingScanner) Scan(_ context.Context, _ int64, _ string) {
	<-b.block
}

func TestAddDirectoryReturnsBeforeScanCompletes(t *testing.T) {
	root := t.TempDir()
	store := newFakeDirectoryStore()
	scanner := &blockingScanner{block: make(chan struct{})}
	svc := NewService(Roots{root}, store, scanner)

	done := make(chan error, 1)
	go func() {
		_, err := svc.AddDirectory(ctx(), root)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AddDirectory: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AddDirectory did not return promptly — looks like it's waiting on the scan")
	}

	close(scanner.block) // let the background goroutine finish, avoid leaking it
}

func TestAddDirectorySuccess(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Jazz")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	store := newFakeDirectoryStore()
	scanner := newRecordingScanner()
	svc := NewService(Roots{root}, store, scanner)

	dir, err := svc.AddDirectory(ctx(), sub)
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}
	if dir.Root != root || dir.Path != sub {
		t.Fatalf("dir = %+v, want Root=%q Path=%q", dir, root, sub)
	}
	if dir.Status != StatusScanning {
		t.Fatalf("Status = %q, want %q", dir.Status, StatusScanning)
	}

	scanner.waitForCall(t)
	if len(scanner.calls) != 1 || scanner.calls[0].directoryID != dir.ID || scanner.calls[0].path != sub {
		t.Fatalf("scanner.calls = %+v, want one call for directory %d at %q", scanner.calls, dir.ID, sub)
	}
}

func TestAddDirectoryRejectsPathOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	store := newFakeDirectoryStore()
	scanner := newRecordingScanner()
	svc := NewService(Roots{root}, store, scanner)

	_, err := svc.AddDirectory(ctx(), outside)
	if !errors.Is(err, ErrOutsideRoots) {
		t.Fatalf("AddDirectory error = %v, want ErrOutsideRoots", err)
	}
	if len(scanner.calls) != 0 {
		t.Fatalf("expected no scan to be started, got %+v", scanner.calls)
	}
}

func TestAddDirectoryRejectsNonexistentPath(t *testing.T) {
	root := t.TempDir()
	store := newFakeDirectoryStore()
	svc := NewService(Roots{root}, store, newRecordingScanner())

	_, err := svc.AddDirectory(ctx(), filepath.Join(root, "nope"))
	if err == nil {
		t.Fatal("AddDirectory(nonexistent path) error = nil, want error")
	}
}

func TestAddDirectoryRejectsFilePath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-dir.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := newFakeDirectoryStore()
	svc := NewService(Roots{root}, store, newRecordingScanner())

	_, err := svc.AddDirectory(ctx(), file)
	if err == nil {
		t.Fatal("AddDirectory(file path) error = nil, want error")
	}
}

func TestAddDirectoryRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	store := newFakeDirectoryStore()
	svc := NewService(Roots{root}, store, newRecordingScanner())

	if _, err := svc.AddDirectory(ctx(), root); err != nil {
		t.Fatalf("first AddDirectory: %v", err)
	}
	_, err := svc.AddDirectory(ctx(), root)
	if !errors.Is(err, ErrDuplicateDirectory) {
		t.Fatalf("second AddDirectory error = %v, want ErrDuplicateDirectory", err)
	}
}

// ctxCapturingScanner records the context Scan was called with and blocks
// until the test releases it, so a test can inspect whether
// RemoveDirectory cancelled that context while the scan was still running.
type ctxCapturingScanner struct {
	started chan struct{}
	block   chan struct{}
	ctx     context.Context
}

func newCtxCapturingScanner() *ctxCapturingScanner {
	return &ctxCapturingScanner{started: make(chan struct{}), block: make(chan struct{})}
}

func (c *ctxCapturingScanner) Scan(ctx context.Context, _ int64, _ string) {
	c.ctx = ctx
	close(c.started)
	<-c.block
}

func TestRemoveDirectoryCancelsInFlightScan(t *testing.T) {
	root := t.TempDir()
	store := newFakeDirectoryStore()
	scanner := newCtxCapturingScanner()
	svc := NewService(Roots{root}, store, scanner)

	dir, err := svc.AddDirectory(ctx(), root)
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}

	select {
	case <-scanner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Scan to start")
	}

	if err := svc.RemoveDirectory(ctx(), dir.ID); err != nil {
		t.Fatalf("RemoveDirectory: %v", err)
	}

	select {
	case <-scanner.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expected the scan's context to be cancelled once its directory was removed")
	}

	close(scanner.block) // let the goroutine finish, avoid leaking it
}

func TestServiceRemoveAndListDelegateToStore(t *testing.T) {
	root := t.TempDir()
	store := newFakeDirectoryStore()
	svc := NewService(Roots{root}, store, newRecordingScanner())

	dir, err := svc.AddDirectory(ctx(), root)
	if err != nil {
		t.Fatalf("AddDirectory: %v", err)
	}

	dirs, err := svc.ListDirectories(ctx())
	if err != nil {
		t.Fatalf("ListDirectories: %v", err)
	}
	if len(dirs) != 1 || dirs[0].ID != dir.ID {
		t.Fatalf("ListDirectories = %+v, want one entry with ID %d", dirs, dir.ID)
	}

	if err := svc.RemoveDirectory(ctx(), dir.ID); err != nil {
		t.Fatalf("RemoveDirectory: %v", err)
	}
	if _, err := svc.store.GetDirectory(ctx(), dir.ID); !errors.Is(err, ErrDirectoryNotFound) {
		t.Fatalf("GetDirectory after remove = %v, want ErrDirectoryNotFound", err)
	}
}
