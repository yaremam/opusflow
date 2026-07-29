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

// TestTrackTokenUsageNeverBlocks covers every shape of request TDR 024
// requires to succeed identically: no token, an unrecognized token, and a
// revoked token all pass through exactly like a valid one — nothing is
// ever gated.
func TestTrackTokenUsageNeverBlocks(t *testing.T) {
	store := testStore(t)
	_, tok, err := NewService(store).CreateToken(ctx(), "Phone")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if err := store.Delete(ctx(), tok.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	cases := []struct {
		name   string
		header string
	}{
		{"no Authorization header", ""},
		{"an unrecognized token", "Bearer complete-nonsense"},
		{"a revoked token", "Bearer whatever-was-just-deleted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler := TrackTokenUsage(store)(okHandler())
			req := httptest.NewRequest("GET", "/api/library/artists", nil)
			if c.header != "" {
				req.Header.Set("Authorization", c.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 with %s — nothing is ever gated (TDR 024)", rec.Code, c.name)
			}
		})
	}
}

// TestTrackTokenUsageRecordsUsageOnAValidToken is the one thing this
// middleware still does: keep Settings' Paired Devices "last used"
// column meaningful.
func TestTrackTokenUsageRecordsUsageOnAValidToken(t *testing.T) {
	store := testStore(t)
	plaintext, tok, err := NewService(store).CreateToken(ctx(), "Phone")
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}

	before, err := store.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if before[0].LastUsedAt != nil {
		t.Fatalf("a freshly created token already has LastUsedAt set: %+v", before[0])
	}

	handler := TrackTokenUsage(store)(okHandler())
	req := httptest.NewRequest("GET", "/api/library/artists", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	after, err := store.List(ctx())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(after) != 1 || after[0].ID != tok.ID || after[0].LastUsedAt == nil {
		t.Fatalf("List after a request with a valid token = %+v, want LastUsedAt set on id %d", after, tok.ID)
	}
}
