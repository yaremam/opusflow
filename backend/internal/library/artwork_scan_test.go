package library

import (
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// testPNG returns minimal-but-decodable PNG bytes, matching the enrich
// package's own test helper — ImageStore.Save must actually decode
// whatever InsertTrack hands it.
func testPNG(t *testing.T) []byte {
	t.Helper()
	// A 1x1 solid-color PNG, hand-encoded once rather than pulling in
	// image/png here just to build a fixture.
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d, 0xb0, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func insertTrackWithArtwork(t *testing.T, s *Store, artist, album string, artwork []byte) {
	t.Helper()
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:    mustCreateImport(t, s),
		Path:        "/music/" + artist + "/" + album + "/track.mp3",
		Title:       "Track",
		Artist:      artist,
		Album:       album,
		ArtworkData: artwork,
	}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
}

func TestInsertTrackSavesEmbeddedArtwork(t *testing.T) {
	s := testStore(t)
	s.SetImages(enrich.NewImageStore(t.TempDir()))

	insertTrackWithArtwork(t, s, "Art Artist", "Art Album", testPNG(t))

	album := findAlbumByTitle(t, s, "Art Album")
	if album.CoverThumbURL == "" || album.CoverURL == "" {
		t.Fatalf("expected cover URLs to be set, got %+v", album)
	}
}

func TestInsertTrackKeepsFirstEmbeddedArtworkAcrossTracks(t *testing.T) {
	s := testStore(t)
	s.SetImages(enrich.NewImageStore(t.TempDir()))

	insertTrackWithArtwork(t, s, "Multi Track Artist", "Multi Track Album", testPNG(t))
	album := findAlbumByTitle(t, s, "Multi Track Album")
	firstThumb := album.CoverThumbURL
	if firstThumb == "" {
		t.Fatal("expected first track's artwork to be saved")
	}

	// A second track for the *same* album, also carrying artwork, must not
	// overwrite what the first track already set (AC-1: first image found
	// wins).
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:    mustCreateImport(t, s),
		Path:        "/music/Multi Track Artist/Multi Track Album/track2.mp3",
		Title:       "Track 2",
		Artist:      "Multi Track Artist",
		Album:       "Multi Track Album",
		ArtworkData: testPNG(t),
	}); err != nil {
		t.Fatalf("InsertTrack (second track): %v", err)
	}

	album = findAlbumByTitle(t, s, "Multi Track Album")
	if album.CoverThumbURL != firstThumb {
		t.Fatalf("CoverThumbURL changed from %q to %q; expected the first track's artwork to be kept", firstThumb, album.CoverThumbURL)
	}
}

func TestInsertTrackWithoutImagesConfiguredSkipsArtworkSilently(t *testing.T) {
	s := testStore(t) // SetImages never called
	insertTrackWithArtwork(t, s, "No Images Artist", "No Images Album", testPNG(t))

	album := findAlbumByTitle(t, s, "No Images Album")
	if album.CoverThumbURL != "" || album.CoverURL != "" {
		t.Fatalf("expected no cover URLs without an ImageStore configured, got %+v", album)
	}
}
