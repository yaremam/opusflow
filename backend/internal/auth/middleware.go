package auth

import (
	"log"
	"net/http"
	"strings"
)

// Middleware enforces token auth on everything it wraps (AC-1) — the
// caller decides which routes that is; see httpserver, which wraps every
// route except GET /health. Until this install has ever been bootstrapped
// (see auth_bootstrap's migration comment), every request is let through
// — Bootstrap itself needs to be reachable before any token exists to
// send. Deliberately keyed on "has this install ever been bootstrapped,"
// not "does a token currently exist": deleting the last remaining token
// must fail closed, not silently reopen the API (see
// TestMiddlewareFailsClosedAfterDeletingTheLastToken).
func Middleware(store *Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bootstrapped, err := store.HasBootstrapped(r.Context())
			if err != nil {
				log.Printf("auth: checking bootstrap marker: %v", err)
				http.Error(w, "server not ready", http.StatusServiceUnavailable)
				return
			}
			if !bootstrapped {
				next.ServeHTTP(w, r)
				return
			}

			token, ok := bearerToken(r)
			if !ok {
				http.Error(w, "missing or malformed Authorization header", http.StatusUnauthorized)
				return
			}

			valid, err := store.ValidateAndTouch(r.Context(), HashToken(token))
			if err != nil {
				log.Printf("auth: validating token: %v", err)
				http.Error(w, "server not ready", http.StatusServiceUnavailable)
				return
			}
			if !valid {
				http.Error(w, "invalid or revoked token", http.StatusUnauthorized)
				return
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
