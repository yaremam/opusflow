package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestMusicBrainz builds a MusicBrainz client pointed at a test server,
// with the rate limiter effectively disabled — these tests exercise
// request-building/response-parsing, not pacing (ratelimit_test.go covers
// that).
func newTestMusicBrainz(t *testing.T, handler http.HandlerFunc) (*MusicBrainz, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	m := NewMusicBrainz("opusflow-test/0.1 (test@example.com)")
	m.baseURL = srv.URL
	m.limiter = newRateLimiter(time.Nanosecond)
	return m, srv
}

func TestSearchArtistReturnsTopMatch(t *testing.T) {
	var gotUserAgent, gotQuery string
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		gotQuery = r.URL.Query().Get("query")
		w.Write([]byte(`{"artists": [{"id": "artist-mbid-1", "name": "Marlow Creek", "score": 100}]}`))
	})

	mbid, found, err := m.SearchArtist(context.Background(), "Marlow Creek")
	if err != nil {
		t.Fatalf("SearchArtist: %v", err)
	}
	if !found || mbid != "artist-mbid-1" {
		t.Fatalf("mbid=%q found=%v", mbid, found)
	}
	if gotUserAgent == "" {
		t.Fatal("expected a User-Agent header per MusicBrainz's usage policy")
	}
	if gotQuery != "Marlow Creek" {
		t.Fatalf("query = %q", gotQuery)
	}
}

func TestSearchArtistNoMatch(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": []}`))
	})

	_, found, err := m.SearchArtist(context.Background(), "Nobody")
	if err != nil {
		t.Fatalf("SearchArtist: %v", err)
	}
	if found {
		t.Fatal("expected found = false for an empty result set")
	}
}

func TestSearchReleaseGroupReturnsTopMatch(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"release-groups": [{"id": "rg-mbid-1", "title": "Night Vessels", "score": 100}]}`))
	})

	mbid, found, err := m.SearchReleaseGroup(context.Background(), "Night Vessels", "Marlow Creek")
	if err != nil {
		t.Fatalf("SearchReleaseGroup: %v", err)
	}
	if !found || mbid != "rg-mbid-1" {
		t.Fatalf("mbid=%q found=%v", mbid, found)
	}
}

func TestLookupArtistParsesFactsAndWikidataRelation(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"life-span": {"begin": "2011-05-01"},
			"area": {"name": "United Kingdom"},
			"genres": [{"name": "Dream pop"}, {"name": "Folk rock"}],
			"relations": [
				{"type": "official homepage", "url": {"resource": "https://example.com"}},
				{"type": "wikidata", "url": {"resource": "https://www.wikidata.org/wiki/Q12345"}}
			]
		}`))
	})

	info, err := m.LookupArtist(context.Background(), "artist-mbid-1")
	if err != nil {
		t.Fatalf("LookupArtist: %v", err)
	}
	if info.FormedYear != 2011 {
		t.Fatalf("FormedYear = %d, want 2011", info.FormedYear)
	}
	if info.Country != "United Kingdom" {
		t.Fatalf("Country = %q", info.Country)
	}
	if len(info.Genres) != 2 || info.Genres[0] != "Dream pop" {
		t.Fatalf("Genres = %+v", info.Genres)
	}
	if info.WikidataURL != "https://www.wikidata.org/wiki/Q12345" {
		t.Fatalf("WikidataURL = %q", info.WikidataURL)
	}
}

func TestLookupArtistWithNoWikidataRelation(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"life-span": {}, "area": {}, "genres": [], "relations": []}`))
	})

	info, err := m.LookupArtist(context.Background(), "artist-mbid-2")
	if err != nil {
		t.Fatalf("LookupArtist: %v", err)
	}
	if info.WikidataURL != "" {
		t.Fatalf("WikidataURL = %q, want empty", info.WikidataURL)
	}
	if info.FormedYear != 0 {
		t.Fatalf("FormedYear = %d, want 0 for missing life-span", info.FormedYear)
	}
}

func TestLookupReleaseGroupParsesLabelCountryFromFirstRelease(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"genres": [{"name": "Dream pop"}],
			"relations": [],
			"releases": [
				{"country": "GB", "label-info": [{"label": {"name": "Harbor & Kite"}}]},
				{"country": "US", "label-info": [{"label": {"name": "Other Label"}}]}
			]
		}`))
	})

	info, err := m.LookupReleaseGroup(context.Background(), "rg-mbid-1")
	if err != nil {
		t.Fatalf("LookupReleaseGroup: %v", err)
	}
	if info.Country != "GB" || info.Label != "Harbor & Kite" {
		t.Fatalf("Country=%q Label=%q, want GB / Harbor & Kite (first release)", info.Country, info.Label)
	}
	if len(info.Genres) != 1 || info.Genres[0] != "Dream pop" {
		t.Fatalf("Genres = %+v", info.Genres)
	}
}

func TestLookupReleaseGroupWithNoReleases(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"genres": [], "relations": [], "releases": []}`))
	})

	info, err := m.LookupReleaseGroup(context.Background(), "rg-mbid-2")
	if err != nil {
		t.Fatalf("LookupReleaseGroup: %v", err)
	}
	if info.Country != "" || info.Label != "" {
		t.Fatalf("expected empty Country/Label with no releases, got %+v", info)
	}
}

func TestMusicBrainzGetPropagatesServerError(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, _, err := m.SearchArtist(context.Background(), "Whatever")
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}
