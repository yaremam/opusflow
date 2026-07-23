package scan

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// fakeProgressStore is an in-memory ProgressStore for testing the Scanner
// without a real database.
type fakeProgressStore struct {
	mu sync.Mutex

	progress   []progressCall
	tracks     []Track
	fileErrors []fileErrorCall
	completed  bool
	failed     bool
	failMsg    string
}

type progressCall struct{ processed, total int }
type fileErrorCall struct{ path, msg string }

func (f *fakeProgressStore) UpdateProgress(_ context.Context, _ int64, processed, total int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress = append(f.progress, progressCall{processed, total})
	return nil
}

func (f *fakeProgressStore) InsertTrack(_ context.Context, t Track) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tracks = append(f.tracks, t)
	return nil
}

func (f *fakeProgressStore) RecordFileError(_ context.Context, _ int64, path, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fileErrors = append(f.fileErrors, fileErrorCall{path, msg})
	return nil
}

func (f *fakeProgressStore) MarkComplete(_ context.Context, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = true
	return nil
}

func (f *fakeProgressStore) MarkFailed(_ context.Context, _ int64, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = true
	f.failMsg = msg
	return nil
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScannerImportsRecognizedFilesRecursively(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Rock"), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), filepath.Join(root, "top.mp3"))
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.flac"), filepath.Join(root, "Rock", "nested.flac"))
	if err := os.WriteFile(filepath.Join(root, "cover.jpg"), []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeProgressStore{}
	s := NewScanner(store)
	s.Scan(context.Background(), 1, root)

	if len(store.tracks) != 2 {
		t.Fatalf("tracks = %+v, want 2 (nested file should be found, cover.jpg should not)", store.tracks)
	}
	if !store.completed {
		t.Fatal("expected MarkComplete to be called")
	}
	if store.failed {
		t.Fatalf("expected no failure, got failMsg=%q", store.failMsg)
	}
	if len(store.fileErrors) != 0 {
		t.Fatalf("fileErrors = %+v, want none", store.fileErrors)
	}

	last := store.progress[len(store.progress)-1]
	if last.processed != 2 || last.total != 2 {
		t.Fatalf("final progress = %+v, want processed=2 total=2", last)
	}
}

func TestScannerRecordsPerFileErrorsAndContinues(t *testing.T) {
	root := t.TempDir()
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), filepath.Join(root, "good.mp3"))

	// A file with a recognized extension but corrupt/truncated ID3v2 data,
	// which ExtractTags treats as a hard error, not a soft fallback.
	corrupt := append([]byte{'I', 'D', '3', 3, 0, 0, 0x7F, 0x7F, 0x7F, 0x7F}, []byte("TIT2")...)
	if err := os.WriteFile(filepath.Join(root, "bad.mp3"), corrupt, 0o644); err != nil {
		t.Fatal(err)
	}

	store := &fakeProgressStore{}
	NewScanner(store).Scan(context.Background(), 1, root)

	if len(store.tracks) != 1 {
		t.Fatalf("tracks = %+v, want 1", store.tracks)
	}
	if len(store.fileErrors) != 1 {
		t.Fatalf("fileErrors = %+v, want 1", store.fileErrors)
	}
	if !store.completed {
		t.Fatal("a per-file error should not prevent MarkComplete")
	}
	if store.failed {
		t.Fatal("a per-file error should not mark the directory failed")
	}
}

func TestScannerMarksFailedWhenRootIsUnreadable(t *testing.T) {
	store := &fakeProgressStore{}
	NewScanner(store).Scan(context.Background(), 1, filepath.Join(t.TempDir(), "does-not-exist"))

	if !store.failed {
		t.Fatal("expected MarkFailed to be called when the root directory can't be read")
	}
	if store.completed {
		t.Fatal("expected MarkComplete not to be called")
	}
}

func TestScannerTreatsUnreadableSubdirectoryAsPerFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: permission bits don't restrict access")
	}

	root := t.TempDir()
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), filepath.Join(root, "good.mp3"))

	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.flac"), filepath.Join(blocked, "unreachable.flac"))
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o755) })

	store := &fakeProgressStore{}
	NewScanner(store).Scan(context.Background(), 1, root)

	if !store.completed {
		t.Fatal("an unreadable subdirectory should not prevent the overall scan from completing")
	}
	if store.failed {
		t.Fatal("an unreadable subdirectory should be tolerated, not treated as a job-level failure")
	}
	if len(store.tracks) != 1 {
		t.Fatalf("tracks = %+v, want 1 (the readable file)", store.tracks)
	}
	if len(store.fileErrors) != 1 {
		t.Fatalf("fileErrors = %+v, want 1 (the blocked subdirectory)", store.fileErrors)
	}
}

func TestScannerEmptyDirectoryCompletesWithNoTracks(t *testing.T) {
	root := t.TempDir()

	store := &fakeProgressStore{}
	NewScanner(store).Scan(context.Background(), 1, root)

	if !store.completed {
		t.Fatal("expected MarkComplete for an empty directory")
	}
	if len(store.tracks) != 0 {
		t.Fatalf("tracks = %+v, want none", store.tracks)
	}
}

func TestScannerRecordsErrorForUnreadableFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: permission bits don't restrict access")
	}

	root := t.TempDir()
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), filepath.Join(root, "good.mp3"))

	blocked := filepath.Join(root, "blocked.mp3")
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), blocked)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(blocked, 0o644) })

	store := &fakeProgressStore{}
	NewScanner(store).Scan(context.Background(), 1, root)

	if !store.completed {
		t.Fatal("an unreadable file should not prevent the overall scan from completing")
	}
	if len(store.tracks) != 1 {
		t.Fatalf("tracks = %+v, want 1 (the readable file)", store.tracks)
	}
	if len(store.fileErrors) != 1 {
		t.Fatalf("fileErrors = %+v, want 1 (the unreadable file)", store.fileErrors)
	}
}

func TestScannerStopsWhenContextCancelled(t *testing.T) {
	// The caller (library.Service) cancels this context when a directory is
	// removed while its scan is still running — Scan must stop without
	// writing any further state for a directory that may no longer exist.
	root := t.TempDir()
	copyFile(t, filepath.Join("testdata", "with_tags", "sample.mp3"), filepath.Join(root, "top.mp3"))

	store := &fakeProgressStore{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	NewScanner(store).Scan(ctx, 1, root)

	if store.completed || store.failed {
		t.Fatalf("a cancelled scan should reach neither terminal state, got completed=%v failed=%v", store.completed, store.failed)
	}
	if len(store.tracks) != 0 {
		t.Fatalf("tracks = %+v, want none from a cancelled scan", store.tracks)
	}
}
