package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/db"
)

func randomSuffix() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// testStore opens a per-test schema against a real Postgres instance,
// migrated fresh, skipping if DATABASE_URL isn't configured — same
// convention as internal/library's testStore.
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}

	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)

	schema := "test_auth_" + randomSuffix()
	if _, err := conn.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { conn.Exec("DROP SCHEMA " + schema + " CASCADE") })
	if _, err := conn.Exec("SET search_path TO " + schema); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	return NewStore(conn)
}

func ctx() context.Context { return context.Background() }

func TestCreateAndList(t *testing.T) {
	s := testStore(t)

	tok, err := s.Create(ctx(), "Kitchen iPad", HashToken("tok-1"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tok.ID == 0 {
		t.Fatalf("Create returned zero ID")
	}
	if tok.Name != "Kitchen iPad" {
		t.Fatalf("Name = %q, want %q", tok.Name, "Kitchen iPad")
	}
	if tok.LastUsedAt != nil {
		t.Fatalf("LastUsedAt = %v, want nil for a never-used token", tok.LastUsedAt)
	}

	list, err := s.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].ID != tok.ID {
		t.Fatalf("List = %+v, want a single row matching %+v", list, tok)
	}
}

func TestValidateAndTouchAcceptsKnownHashAndSetsLastUsed(t *testing.T) {
	s := testStore(t)
	hash := HashToken("tok-valid")
	if _, err := s.Create(ctx(), "Phone", hash); err != nil {
		t.Fatalf("Create: %v", err)
	}

	valid, err := s.ValidateAndTouch(ctx(), hash)
	if err != nil {
		t.Fatalf("ValidateAndTouch: %v", err)
	}
	if !valid {
		t.Fatalf("ValidateAndTouch = false, want true for a known hash")
	}

	list, err := s.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list[0].LastUsedAt == nil {
		t.Fatalf("LastUsedAt still nil after ValidateAndTouch")
	}
}

func TestValidateAndTouchRejectsUnknownHash(t *testing.T) {
	s := testStore(t)
	valid, err := s.ValidateAndTouch(ctx(), HashToken("never-created"))
	if err != nil {
		t.Fatalf("ValidateAndTouch: %v", err)
	}
	if valid {
		t.Fatalf("ValidateAndTouch = true, want false for an unknown hash")
	}
}

func TestDeleteRevokesToken(t *testing.T) {
	s := testStore(t)
	hash := HashToken("tok-to-revoke")
	tok, err := s.Create(ctx(), "Old Phone", hash)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(ctx(), tok.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	valid, err := s.ValidateAndTouch(ctx(), hash)
	if err != nil {
		t.Fatalf("ValidateAndTouch: %v", err)
	}
	if valid {
		t.Fatalf("ValidateAndTouch = true after Delete, want false")
	}

	list, err := s.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("List = %+v, want empty after Delete", list)
	}
}

func TestDeleteUnknownIDIsNotFound(t *testing.T) {
	s := testStore(t)
	err := s.Delete(ctx(), 999999)
	if err != ErrTokenNotFound {
		t.Fatalf("Delete unknown ID: got %v, want ErrTokenNotFound", err)
	}
}
