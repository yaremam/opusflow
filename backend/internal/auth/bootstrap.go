package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// BootstrapFileName is where Bootstrap writes the first token's plaintext
// — under DATA_DIR, the same volume every compose file already mounts, so
// an operator doesn't need to configure anything new to find it.
const BootstrapFileName = ".opusflow_admin_token"

// Bootstrap is a no-op once any token exists (AC-3) — it only ever runs
// on a genuinely fresh install, called after migrations succeed (see
// cmd/server). On an empty store it creates one token named "Bootstrap"
// and writes its plaintext to dataDir/BootstrapFileName with 0600
// permissions — the same filesystem-trust posture this repo already uses
// for Postgres (no password, POSTGRES_HOST_AUTH_METHOD=trust) rather than
// a second secret-management mechanism.
func Bootstrap(ctx context.Context, store *Store, dataDir string) (created bool, err error) {
	n, err := store.Count(ctx)
	if err != nil {
		return false, fmt.Errorf("checking for existing tokens: %w", err)
	}
	if n > 0 {
		return false, nil
	}

	svc := NewService(store)
	plaintext, _, err := svc.CreateToken(ctx, "Bootstrap")
	if err != nil {
		return false, fmt.Errorf("creating bootstrap token: %w", err)
	}

	// CreateToken already marks this install bootstrapped as a side effect
	// of creating its first-ever token — see its own doc comment.
	path := filepath.Join(dataDir, BootstrapFileName)
	if err := os.WriteFile(path, []byte(plaintext), 0o600); err != nil {
		return false, fmt.Errorf("writing bootstrap token file: %w", err)
	}
	return true, nil
}
