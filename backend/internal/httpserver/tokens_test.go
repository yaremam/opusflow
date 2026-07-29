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

func TestAPIOpenDuringBootstrapWindow(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc, authStore).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with no token configured yet, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAPIRejectsMissingTokenOnceBootstrapped(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	if _, err := auth.Bootstrap(t.Context(), authStore, t.TempDir()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc, authStore).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHealthAndArtworkStayOpenEvenWhenBootstrapped(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	if _, err := auth.Bootstrap(t.Context(), authStore, t.TempDir()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	handler := New("", "", "", "", svc, authStore)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200 (never gated)", rec.Code)
	}
}

func TestAPIHealthRequiresAuthOnceBootstrapped(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	if _, err := auth.Bootstrap(t.Context(), authStore, t.TempDir()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	New("", "", "", "", svc, authStore).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/health status = %d, want 401 without a token — this is exactly what mobile's pairing check relies on", rec.Code)
	}
}

func TestCreateListAndRevokeTokenEndToEnd(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	handler := New("", "", "", "", svc, authStore)

	// Create: works unauthenticated during the bootstrap window, matching
	// the very first token a fresh install ever makes through the UI —
	// though in practice AC-3's file-based Bootstrap wins the race in
	// production; this exercises the endpoint directly.
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

	// A second token so revoking the first below isn't revoking the last
	// one remaining — that's TestDeleteTokenReturnsConflictWhenItsTheLastOne's
	// job.
	secondReq := httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"Tablet"}`))
	secondReq.Header.Set("Authorization", "Bearer "+created.Token)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/tokens (second token) status = %d, want 201, body = %s", secondRec.Code, secondRec.Body.String())
	}

	// The token this endpoint just minted now gates every other /api/* call.
	authedReq := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	authedReq.Header.Set("Authorization", "Bearer "+created.Token)
	authedRec := httptest.NewRecorder()
	handler.ServeHTTP(authedRec, authedReq)
	if authedRec.Code != http.StatusOK {
		t.Fatalf("GET /api/library/artists with the freshly-created token: status = %d, want 200, body = %s", authedRec.Code, authedRec.Body.String())
	}

	// List: reflects the created token, never its hash/plaintext again.
	listReq := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	listReq.Header.Set("Authorization", "Bearer "+created.Token)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/tokens status = %d, want 200, body = %s", listRec.Code, listRec.Body.String())
	}
	var list []tokenResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list = %+v, want two rows", list)
	}
	if strings.Contains(listRec.Body.String(), created.Token) {
		t.Fatalf("GET /api/tokens response leaked the plaintext token")
	}

	// Revoke: the token that gated everything above stops working
	// immediately.
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+strconv.FormatInt(created.ID, 10), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+created.Token)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/tokens/{id} status = %d, want 204, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	revokedReq := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	revokedReq.Header.Set("Authorization", "Bearer "+created.Token)
	revokedRec := httptest.NewRecorder()
	handler.ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/library/artists with the just-revoked token: status = %d, want 401", revokedRec.Code)
	}
}

// TestDeleteTokenReturnsConflictWhenItsTheLastOne guards against issue #59:
// a household admin could delete every pairing token, including the one
// they were using, and lock themselves (and everyone else) out of the
// entire app with no way back in short of a database edit.
func TestDeleteTokenReturnsConflictWhenItsTheLastOne(t *testing.T) {
	svc, authStore := testServiceAndAuth(t)
	handler := New("", "", "", "", svc, authStore)

	createReq := httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(`{"name":"Only Device"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/tokens status = %d, want 201, body = %s", createRec.Code, createRec.Body.String())
	}
	var created createTokenResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/tokens/"+strconv.FormatInt(created.ID, 10), nil)
	deleteReq.Header.Set("Authorization", "Bearer "+created.Token)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusConflict {
		t.Fatalf("DELETE /api/tokens/{id} on the last remaining token: status = %d, want 409, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	// The token must still work — the refused delete didn't half-apply.
	stillWorksReq := httptest.NewRequest(http.MethodGet, "/api/library/artists", nil)
	stillWorksReq.Header.Set("Authorization", "Bearer "+created.Token)
	stillWorksRec := httptest.NewRecorder()
	handler.ServeHTTP(stillWorksRec, stillWorksReq)
	if stillWorksRec.Code != http.StatusOK {
		t.Fatalf("GET /api/library/artists after a refused delete: status = %d, want 200, body = %s", stillWorksRec.Code, stillWorksRec.Body.String())
	}
}
