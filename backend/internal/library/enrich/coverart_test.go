package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoverArtArchiveFetchAllReturnsEveryImageWithItsType(t *testing.T) {
	frontBytes := testPNG(t, 500, 500)
	backBytes := testPNG(t, 400, 400)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// The real response embeds full absolute image URLs (potentially on a
	// different host than the release-group endpoint itself) — the mux
	// route below mirrors that by pointing at this same test server.
	mux.HandleFunc("/release-group/rg-1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": [
			{"types": ["Front"], "image": "` + srv.URL + `/front.png"},
			{"types": ["Back"], "image": "` + srv.URL + `/back.png"}
		]}`))
	})
	mux.HandleFunc("/front.png", func(w http.ResponseWriter, r *http.Request) { w.Write(frontBytes) })
	mux.HandleFunc("/back.png", func(w http.ResponseWriter, r *http.Request) { w.Write(backBytes) })

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	images, err := c.FetchAll(context.Background(), "rg-1")
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(images) != 2 {
		t.Fatalf("len(images) = %d, want 2", len(images))
	}
	if images[0].PictureType != "front" || len(images[0].Data) != len(frontBytes) {
		t.Fatalf("images[0] = type=%q len=%d, want type=front len=%d", images[0].PictureType, len(images[0].Data), len(frontBytes))
	}
	if images[1].PictureType != "back" || len(images[1].Data) != len(backBytes) {
		t.Fatalf("images[1] = type=%q len=%d, want type=back len=%d", images[1].PictureType, len(images[1].Data), len(backBytes))
	}
}

func TestCoverArtArchiveFetchAllHandlesUntypedImage(t *testing.T) {
	imageBytes := testPNG(t, 300, 300)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/release-group/rg-untyped", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": [{"types": [], "image": "` + srv.URL + `/img.png"}]}`))
	})
	mux.HandleFunc("/img.png", func(w http.ResponseWriter, r *http.Request) { w.Write(imageBytes) })

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	images, err := c.FetchAll(context.Background(), "rg-untyped")
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(images) != 1 || images[0].PictureType != "" {
		t.Fatalf("images = %+v, want 1 image with blank picture type", images)
	}
}

func TestCoverArtArchiveFetchAllNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	images, err := c.FetchAll(context.Background(), "rg-missing")
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("images = %+v, want none for a 404", images)
	}
}

func TestCoverArtArchiveFetchAllEmptyImagesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": []}`))
	}))
	t.Cleanup(srv.Close)

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	images, err := c.FetchAll(context.Background(), "rg-empty")
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(images) != 0 {
		t.Fatalf("images = %+v, want none", images)
	}
}

func TestCoverArtArchiveFetchAllSkipsImageThatFailsToDownload(t *testing.T) {
	goodBytes := testPNG(t, 200, 200)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/release-group/rg-partial", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": [
			{"types": ["Front"], "image": "` + srv.URL + `/missing.png"},
			{"types": ["Back"], "image": "` + srv.URL + `/good.png"}
		]}`))
	})
	mux.HandleFunc("/missing.png", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	mux.HandleFunc("/good.png", func(w http.ResponseWriter, r *http.Request) { w.Write(goodBytes) })

	c := NewCoverArtArchive("opusflow-test/0.1")
	c.baseURL = srv.URL

	images, err := c.FetchAll(context.Background(), "rg-partial")
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(images) != 1 || images[0].PictureType != "back" {
		t.Fatalf("images = %+v, want just the back image that downloaded successfully", images)
	}
}
