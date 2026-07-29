package auth

import (
	"log"
	"net/http"
	"strings"
)

// TrackTokenUsage never blocks a request (TDR 024 — opusflow no longer
// gates anything, see docs/tdr/024_drop_web_auth_gate_design.md). Its one
// remaining job: if a request carries a Bearer token that matches a real
// row, record that it was used, so Settings' Paired Devices list's "last
// used" column stays meaningful. A missing, malformed, or unrecognized
// token is not an error — it's just not tracked.
func TrackTokenUsage(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token, ok := bearerToken(r); ok {
				if _, err := store.ValidateAndTouch(r.Context(), HashToken(token)); err != nil {
					log.Printf("auth: recording token usage: %v", err)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}
