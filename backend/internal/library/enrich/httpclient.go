package enrich

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// httpTimeout bounds every enrich network client's requests — MusicBrainz,
// Cover Art Archive, and Wikidata/Wikipedia each make one external call at
// a time, none expected to take long.
const httpTimeout = 10 * time.Second

// newHTTPClient builds the *http.Client every enrich network client shares.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: httpTimeout}
}

// doGet sends a GET request to reqURL with a descriptive User-Agent
// (required by MusicBrainz/Wikidata's usage policies) and, when accept is
// non-empty, an Accept header — the one piece of request-building every
// enrich network client repeats identically. Callers own everything about
// interpreting the response (which status codes count as "not found" vs.
// an error, JSON vs. raw bytes) since that differs enough between clients
// that folding it in here would trade a little duplication for a lot of
// indirection.
func doGet(ctx context.Context, client *http.Client, userAgent, accept, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	return resp, nil
}
