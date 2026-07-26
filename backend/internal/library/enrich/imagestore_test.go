package enrich

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// testPNG builds an in-memory solid-color PNG of the given size — a stand-in
// for embedded tag art / a fetched Cover Art Archive image, without needing
// a real file or network fetch.
func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encoding test PNG: %v", err)
	}
	return buf.Bytes()
}

func decodedSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decoding config of %s: %v", path, err)
	}
	return cfg.Width, cfg.Height
}

func TestImageStoreSaveResizesAndReturnsURLs(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	thumbURL, fullURL, err := st.Save("album", 42, testPNG(t, 1400, 1400))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if thumbURL != "/artwork/album/42/thumb.jpg" || fullURL != "/artwork/album/42/full.jpg" {
		t.Fatalf("thumbURL=%q fullURL=%q", thumbURL, fullURL)
	}

	tw, th := decodedSize(t, dir+"/album/42/thumb.jpg")
	if tw != thumbSize || th != thumbSize {
		t.Fatalf("thumb dims = %dx%d, want %dx%d", tw, th, thumbSize, thumbSize)
	}
	fw, fh := decodedSize(t, dir+"/album/42/full.jpg")
	if fw != fullSize || fh != fullSize {
		t.Fatalf("full dims = %dx%d, want %dx%d", fw, fh, fullSize, fullSize)
	}
}

func TestImageStoreSaveDoesNotUpscaleSmallSource(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	_, fullURL, err := st.Save("artist", 7, testPNG(t, 120, 120))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	fw, fh := decodedSize(t, dir+fullURL[len("/artwork"):])
	if fw != 120 || fh != 120 {
		t.Fatalf("full dims = %dx%d, want unchanged 120x120 (no upscale)", fw, fh)
	}
}

func TestImageStoreSavePreservesAspectRatio(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	_, _, err := st.Save("album", 1, testPNG(t, 2000, 1000))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	fw, fh := decodedSize(t, dir+"/album/1/full.jpg")
	if fw != fullSize || fh != fullSize/2 {
		t.Fatalf("full dims = %dx%d, want %dx%d (2:1 preserved)", fw, fh, fullSize, fullSize/2)
	}
}

func TestImageStoreSaveRejectsUndecodableData(t *testing.T) {
	st := NewImageStore(t.TempDir())
	if _, _, err := st.Save("album", 1, []byte("not an image")); err == nil {
		t.Fatal("expected an error for undecodable image data")
	}
}
