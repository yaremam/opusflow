package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestWikidata(t *testing.T, apiHandler, commonsHandler, wikipediaHandler http.HandlerFunc) *Wikidata {
	t.Helper()
	mux := http.NewServeMux()
	if apiHandler != nil {
		mux.HandleFunc("/api", apiHandler)
	}
	if commonsHandler != nil {
		mux.HandleFunc("/commons/", commonsHandler)
	}
	if wikipediaHandler != nil {
		mux.HandleFunc("/wikipedia/", wikipediaHandler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	w := NewWikidata("opusflow-test/0.1")
	w.apiBaseURL = srv.URL + "/api"
	w.commonsBaseURL = srv.URL + "/commons"
	w.wikipediaBaseURL = srv.URL + "/wikipedia"
	return w
}

func TestResolveEntityExtractsImageAndSitelink(t *testing.T) {
	var gotIDs string
	w := newTestWikidata(t, func(rw http.ResponseWriter, r *http.Request) {
		gotIDs = r.URL.Query().Get("ids")
		rw.Write([]byte(`{
			"entities": {
				"Q12345": {
					"sitelinks": {"enwiki": {"title": "Marlow Creek (band)"}},
					"claims": {"P18": [{"mainsnak": {"datavalue": {"value": "Marlow Creek band photo.jpg"}}}]}
				}
			}
		}`))
	}, nil, nil)

	entity, err := w.ResolveEntity(context.Background(), "https://www.wikidata.org/wiki/Q12345")
	if err != nil {
		t.Fatalf("ResolveEntity: %v", err)
	}
	if gotIDs != "Q12345" {
		t.Fatalf("ids param = %q", gotIDs)
	}
	if entity.ImageFilename != "Marlow Creek band photo.jpg" {
		t.Fatalf("ImageFilename = %q", entity.ImageFilename)
	}
	if entity.WikipediaTitle != "Marlow Creek (band)" {
		t.Fatalf("WikipediaTitle = %q", entity.WikipediaTitle)
	}
}

func TestResolveEntityWithNoImageOrSitelink(t *testing.T) {
	w := newTestWikidata(t, func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(`{"entities": {"Q999": {"sitelinks": {}, "claims": {}}}}`))
	}, nil, nil)

	entity, err := w.ResolveEntity(context.Background(), "https://www.wikidata.org/wiki/Q999")
	if err != nil {
		t.Fatalf("ResolveEntity: %v", err)
	}
	if entity.ImageFilename != "" || entity.WikipediaTitle != "" {
		t.Fatalf("expected empty Entity, got %+v", entity)
	}
}

func TestResolveEntityRejectsURLWithNoQID(t *testing.T) {
	w := newTestWikidata(t, nil, nil, nil)
	if _, err := w.ResolveEntity(context.Background(), "https://example.com/not-wikidata"); err == nil {
		t.Fatal("expected an error for a URL with no QID")
	}
}

func TestFetchImageFollowsRedirect(t *testing.T) {
	imageBytes := testPNG(t, 300, 300)
	w := newTestWikidata(t, nil, func(rw http.ResponseWriter, r *http.Request) {
		rw.Write(imageBytes)
	}, nil)

	data, err := w.FetchImage(context.Background(), "Some Photo.jpg")
	if err != nil {
		t.Fatalf("FetchImage: %v", err)
	}
	if len(data) != len(imageBytes) {
		t.Fatalf("got %d bytes, want %d", len(data), len(imageBytes))
	}
}

func TestFetchSummaryReturnsExtractAndURL(t *testing.T) {
	w := newTestWikidata(t, nil, nil, func(rw http.ResponseWriter, r *http.Request) {
		rw.Write([]byte(`{
			"extract": "Night Vessels was recorded over a single winter.",
			"content_urls": {"desktop": {"page": "https://en.wikipedia.org/wiki/Night_Vessels"}}
		}`))
	})

	summary, err := w.FetchSummary(context.Background(), "Night Vessels")
	if err != nil {
		t.Fatalf("FetchSummary: %v", err)
	}
	if summary.Extract != "Night Vessels was recorded over a single winter." {
		t.Fatalf("Extract = %q", summary.Extract)
	}
	if summary.URL != "https://en.wikipedia.org/wiki/Night_Vessels" {
		t.Fatalf("URL = %q", summary.URL)
	}
}

func TestFetchSummaryNotFound(t *testing.T) {
	w := newTestWikidata(t, nil, nil, func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	})

	summary, err := w.FetchSummary(context.Background(), "No Such Article")
	if err != nil {
		t.Fatalf("FetchSummary: %v", err)
	}
	if summary.Extract != "" || summary.URL != "" {
		t.Fatalf("expected empty Summary for 404, got %+v", summary)
	}
}
