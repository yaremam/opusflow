package auth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBootstrapCreatesTokenAndWritesFileOnEmptyStore(t *testing.T) {
	store := testStore(t)
	dataDir := t.TempDir()

	created, err := Bootstrap(ctx(), store, dataDir)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if !created {
		t.Fatalf("Bootstrap created = false, want true on an empty store")
	}

	n, err := store.Count(ctx())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("Count after Bootstrap = %d, want 1", n)
	}

	path := filepath.Join(dataDir, BootstrapFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat bootstrap file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("bootstrap file mode = %o, want 0600", perm)
	}

	plaintext, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading bootstrap file: %v", err)
	}
	valid, err := store.ValidateAndTouch(ctx(), HashToken(string(plaintext)))
	if err != nil {
		t.Fatalf("ValidateAndTouch: %v", err)
	}
	if !valid {
		t.Fatalf("bootstrap file's token doesn't validate against what Bootstrap stored")
	}
}

func TestBootstrapIsNoOpWhenATokenAlreadyExists(t *testing.T) {
	store := testStore(t)
	dataDir := t.TempDir()

	if _, err := store.Create(ctx(), "Existing", HashToken("already-here")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	created, err := Bootstrap(ctx(), store, dataDir)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if created {
		t.Fatalf("Bootstrap created = true, want false when a token already exists")
	}

	if _, err := os.Stat(filepath.Join(dataDir, BootstrapFileName)); !os.IsNotExist(err) {
		t.Fatalf("bootstrap file exists when Bootstrap should have been a no-op: err=%v", err)
	}

	n, err := store.Count(ctx())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("Count = %d, want 1 (unchanged)", n)
	}
}
