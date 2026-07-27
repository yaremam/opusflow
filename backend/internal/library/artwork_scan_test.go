package library

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
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

// testPNG2 is a second, genuinely distinct decodable PNG (a different
// solid color) — content-hash dedup (TDR 014 AC-5) must treat this as
// different from testPNG, not collapse it away.
func testPNG2(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 10, G: 200, B: 30, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

func insertTrackWithArtwork(t *testing.T, s *Store, artist, album string, artwork []byte) {
	t.Helper()
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:        mustCreateImport(t, s),
		Path:            "/music/" + artist + "/" + album + "/track.mp3",
		Title:           "Track",
		Artist:          artist,
		Album:           album,
		ArtworkPictures: []organize.EmbeddedPicture{{Data: artwork}},
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

func TestInsertTrackDedupesIdenticalEmbeddedArtworkAcrossTracks(t *testing.T) {
	s := testStore(t)
	s.SetImages(enrich.NewImageStore(t.TempDir()))

	insertTrackWithArtwork(t, s, "Multi Track Artist", "Multi Track Album", testPNG(t))
	album := findAlbumByTitle(t, s, "Multi Track Album")
	firstThumb := album.CoverThumbURL
	if firstThumb == "" {
		t.Fatal("expected first track's artwork to be saved")
	}

	// A second track for the *same* album, carrying the identical embedded
	// picture, must dedupe by content hash (AC-5) rather than adding a
	// second gallery entry — the primary (and its thumbnail) stays exactly
	// what the first track set.
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:        mustCreateImport(t, s),
		Path:            "/music/Multi Track Artist/Multi Track Album/track2.mp3",
		Title:           "Track 2",
		Artist:          "Multi Track Artist",
		Album:           "Multi Track Album",
		ArtworkPictures: []organize.EmbeddedPicture{{Data: testPNG(t)}},
	}); err != nil {
		t.Fatalf("InsertTrack (second track): %v", err)
	}

	detail, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if detail.CoverThumbURL != firstThumb {
		t.Fatalf("CoverThumbURL changed from %q to %q; expected the deduped artwork to stay primary", firstThumb, detail.CoverThumbURL)
	}
	if len(detail.Covers) != 1 {
		t.Fatalf("Covers = %+v, want exactly 1 (identical picture deduped, not added again)", detail.Covers)
	}
}

func TestInsertTrackAddsDifferentEmbeddedArtworkFromSecondTrack(t *testing.T) {
	s := testStore(t)
	s.SetImages(enrich.NewImageStore(t.TempDir()))

	insertTrackWithArtwork(t, s, "Gallery Artist", "Gallery Album", testPNG(t))
	album := findAlbumByTitle(t, s, "Gallery Album")

	// A second track carrying a *different* embedded picture (e.g. a back
	// cover on one track, front on another) must be added as a new gallery
	// entry (AC-7) rather than being ignored because the album's art is
	// already settled.
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:        mustCreateImport(t, s),
		Path:            "/music/Gallery Artist/Gallery Album/track2.mp3",
		Title:           "Track 2",
		Artist:          "Gallery Artist",
		Album:           "Gallery Album",
		ArtworkPictures: []organize.EmbeddedPicture{{Data: testPNG2(t), PictureType: "back"}},
	}); err != nil {
		t.Fatalf("InsertTrack (second track): %v", err)
	}

	detail, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if len(detail.Covers) != 2 {
		t.Fatalf("Covers = %+v, want 2 (both distinct embedded pictures added)", detail.Covers)
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
