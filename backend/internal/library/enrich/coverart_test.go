package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverArtArchiveFetchFrontFollowsRedirectToImage(t *testing.T) {
	imageBytes := testPNG(t, 500, 500)

	mux := http.NewServeMux()
	mux.HandleFunc("/release-group/rg-1/front", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/image/rg-1.png", http.StatusFound)
	})
	mux.HandleFunc("/image/rg-1.png", func(w http.ResponseWriter, r *http.Request) {
		w.Write(imageBytes)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	data, found, err := c.FetchFront(context.Background(), "rg-1")
	if err != nil {
		t.Fatalf("FetchFront: %v", err)
	}
	if !found {
		t.Fatal("expected found = true")
	}
	if len(data) != len(imageBytes) {
		t.Fatalf("got %d bytes, want %d", len(data), len(imageBytes))
	}
}

func TestCoverArtArchiveFetchFrontNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	_, found, err := c.FetchFront(context.Background(), "rg-missing")
	if err != nil {
		t.Fatalf("FetchFront: %v", err)
	}
	if found {
		t.Fatal("expected found = false for a 404")
	}
}
