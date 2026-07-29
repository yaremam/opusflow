package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// bootstrapAndCreate puts store into the only state a real deployment can
// reach once a token exists: bootstrapped, plus a named token — a token
// can only ever come to exist via Bootstrap or an already-authenticated
// request, never in isolation.
func bootstrapAndCreate(t *testing.T, store *Store, name, token string) {
	t.Helper()
	if _, err := Bootstrap(ctx(), store, t.TempDir()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if _, err := store.Create(ctx(), name, HashToken(token)); err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestMiddlewareAllowsAllRequestsWhenNoTokenExistsYet(t *testing.T) {
	store := testStore(t)
	handler := Middleware(store)(okHandler())

	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 during the bootstrap window (no tokens yet)", rec.Code)
	}
}

func TestMiddlewareRejectsMissingTokenOnceOneExists(t *testing.T) {
	store := testStore(t)
	bootstrapAndCreate(t, store, "Phone", "real-token")
	handler := Middleware(store)(okHandler())

	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 with no Authorization header once a token exists", rec.Code)
	}
}

func TestMiddlewareRejectsWrongToken(t *testing.T) {
	store := testStore(t)
	bootstrapAndCreate(t, store, "Phone", "real-token")
	handler := Middleware(store)(okHandler())

	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a wrong token", rec.Code)
	}
}

func TestMiddlewareAcceptsValidToken(t *testing.T) {
	store := testStore(t)
	bootstrapAndCreate(t, store, "Phone", "real-token")
	handler := Middleware(store)(okHandler())

	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	req.Header.Set("Authorization", "Bearer real-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a valid token", rec.Code)
	}
}

func TestMiddlewareRejectsRevokedToken(t *testing.T) {
	store := testStore(t)
	bootstrapAndCreate(t, store, "Phone", "real-token")

	list, err := store.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var phoneID int64
	for _, tok := range list {
		if tok.Name == "Phone" {
			phoneID = tok.ID
		}
	}
	if err := store.Delete(ctx(), phoneID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	handler := Middleware(store)(okHandler())
	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	req.Header.Set("Authorization", "Bearer real-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 for a revoked token", rec.Code)
	}
}

// TestMiddlewareFailsClosedAfterDeletingTheLastToken is the scenario a
// live Count()==0 check would get wrong: in the real system a token can
// only ever come to exist via Bootstrap or an already-authenticated
// request, so deleting every token can't mean "never configured" — it
// must still reject everything until the next restart re-bootstraps.
func TestMiddlewareFailsClosedAfterDeletingTheLastToken(t *testing.T) {
	store := testStore(t)
	dataDir := t.TempDir()
	if _, err := Bootstrap(ctx(), store, dataDir); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	list, err := store.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := store.Delete(ctx(), list[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	n, err := store.Count(ctx())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 0 {
		t.Fatalf("Count = %d, want 0 after deleting the only token", n)
	}

	handler := Middleware(store)(okHandler())
	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — api_tokens is empty again, but this instance was already bootstrapped", rec.Code)
	}
}
