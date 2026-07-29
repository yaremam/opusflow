package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/auth"
	"github.com/yaremam/opusflow/backend/internal/db"
	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// testServiceAndAuth is testStoreAndService plus the *auth.Store backed by
// the same migrated schema, for tests exercising New's authStore
// parameter.
func testServiceAndAuth(t *testing.T) (*library.Service, *auth.Store) {
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

	schema := "test_httpserver_auth_" + randomSuffix()
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

	libStore := library.NewStore(conn)
	return library.NewService(libStore, organize.CopyJob{}), auth.NewStore(conn)
}

// TestAPIIsAlwaysOpen is TDR 024's core behavior: no /api/* route ever
// gates on a token, regardless of whether one exists, is missing, or is
// garbage.
func TestAPIIsAlwaysOpen(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	handler := New("", "", "", "", svc, authStore)

	cases := []struct {
		name   string
		header string
	}{
		{"no token at all", ""},
		{"a token that doesn't exist", "Bearer complete-nonsense"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 with %s, body = %s", rec.Code, c.name, rec.Body.String())
			}
		})
	}
}

func TestHealthAndAPIHealthBothStayOpen(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	handler := New("", "", "", "", svc, authStore)

	for _, path := range []string{"/health", "/api/health"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200 (never gated)", path, rec.Code)
		}
	}
}

func TestCreateListAndRevokeTokenEndToEnd(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	handler := New("", "", "", "", svc, authStore)

	// Create: no auth needed — /api/tokens is exactly as open as
	// everything else now.
	createReq := httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"Kitchen iPad"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/tokens status = %d, want 201, body = %s", createRec.Code, createRec.Body.String())
	}
	var created createTokenResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Token == "" {
		t.Fatalf("response missing plaintext token: %s", createRec.Body.String())
	}
	if created.Name != "Kitchen iPad" {
		t.Fatalf("Name = %q, want %q", created.Name, "Kitchen iPad")
	}

	// A request bearing the token still succeeds (nothing's gated) and
	// still records last-used, so the Paired Devices list stays useful.
	authedReq := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	authedReq.Header.Set("Authorization", "Bearer "+created.Token)
	authedRec := httptest.NewRecorder()
	handler.ServeHTTP(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("GET /api/library/artists with the freshly-created token: status = %d, want 200, body = %s", authedRec.Code, authedRec.Body.String())
	}

	// List: reflects the created token, never its hash/plaintext again,
	// and now shows LastUsedAt from the request above.
	listReq := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/tokens status = %d, want 200, body = %s", listRec.Code, listRec.Body.String())
	}
	var list []tokenResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v, want one row matching id %d", list, created.ID)
	}
	if list[0].LastUsedAt == nil {
		t.Fatalf("list[0].LastUsedAt = nil, want it set after a request bearing this token")
	}
	if strings.Contains(listRec.Body.String(), created.Token) {
		t.Fatalf("GET /api/tokens response leaked the plaintext token")
	}

	// Revoke: no auth needed either, and it's the *only* token — TDR 024
	// removed the last-token-deletion guard (issue #59) since nothing can
	// lock anyone out anymore.
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+strconv.FormatInt(created.ID, 10), nil)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/tokens/{id} status = %d, want 204, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	// The API stays open even with zero tokens left and even using the
	// now-revoked token — revocation only affects that row's bookkeeping,
	// not access.
	revokedReq := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+created.Token)
	revokedRec := httptest.NewRecorder()
	handler.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusOK {
		t.Fatalf("GET /api/library/artists with a just-revoked token: status = %d, want 200 (nothing gates on it)", revokedRec.Code)
	}
}
