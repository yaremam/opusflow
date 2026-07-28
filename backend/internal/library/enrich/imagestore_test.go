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

	thumbURL, fullURL, hash, err := st.Save("album", 42, testPNG(t, 1400, 1400))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	wantThumb := "/artwork/album/42/" + hash + "/thumb.jpg"
	wantFull := "/artwork/album/42/" + hash + "/full.jpg"
	if thumbURL != wantThumb || fullURL != wantFull {
		t.Fatalf("thumbURL=%q fullURL=%q, want %q / %q", thumbURL, fullURL, wantThumb, wantFull)
	}
	if hash == "" {
		t.Fatal("expected a non-empty content hash")
	}

	tw, th := decodedSize(t, dir+"/album/42/"+hash+"/thumb.jpg")
	if tw != thumbSize || th != thumbSize {
		t.Fatalf("thumb dims = %dx%d, want %dx%d", tw, th, thumbSize, thumbSize)
	}
	fw, fh := decodedSize(t, dir+"/album/42/"+hash+"/full.jpg")
	if fw != fullSize || fh != fullSize {
		t.Fatalf("full dims = %dx%d, want %dx%d", fw, fh, fullSize, fullSize)
	}
}

func TestImageStoreSaveDoesNotUpscaleSmallSource(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	_, fullURL, _, err := st.Save("artist", 7, testPNG(t, 120, 120))
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

	_, _, hash, err := st.Save("album", 1, testPNG(t, 2000, 1000))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	fw, fh := decodedSize(t, dir+"/album/1/"+hash+"/full.jpg")
	if fw != fullSize || fh != fullSize/2 {
		t.Fatalf("full dims = %dx%d, want %dx%d (2:1 preserved)", fw, fh, fullSize, fullSize/2)
	}
}

func TestImageStoreSaveRejectsUndecodableData(t *testing.T) {
	st := NewImageStore(t.TempDir())
	if _, _, _, err := st.Save("album", 1, []byte("not an image")); err == nil {
		t.Fatal("expected an error for undecodable image data")
	}
}

func TestImageStoreSaveIsContentAddressed(t *testing.T) {
	st := NewImageStore(t.TempDir())

	_, _, hashA, err := st.Save("album", 1, testPNG(t, 400, 400))
	if err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	_, _, hashB, err := st.Save("album", 1, testPNG(t, 400, 400))
	if err != nil {
		t.Fatalf("Save #2: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("hashA=%q hashB=%q, want identical bytes to produce the same content hash", hashA, hashB)
	}

	_, _, hashC, err := st.Save("album", 1, testPNG(t, 400, 401))
	if err != nil {
		t.Fatalf("Save #3: %v", err)
	}
	if hashC == hashA {
		t.Fatal("expected different image bytes to produce a different content hash")
	}
}

func TestImageStoreDeleteRemovesFilesAndHashDir(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	thumbURL, fullURL, hash, err := st.Save("artist", 3, testPNG(t, 200, 200))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	hashDir := dir + "/artist/3/" + hash

	if err := st.Delete(thumbURL, fullURL); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(hashDir); !os.IsNotExist(err) {
		t.Fatalf("hash dir %q still exists after Delete", hashDir)
	}
}

func TestImageStoreDeleteIgnoresBlankURLs(t *testing.T) {
	st := NewImageStore(t.TempDir())
	if err := st.Delete("", ""); err != nil {
		t.Fatalf("Delete(\"\", \"\") = %v, want nil", err)
	}
}

func TestImageStoreDeleteAllRemovesEveryHashDir(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	if _, _, _, err := st.Save("artist", 3, testPNG(t, 200, 200)); err != nil {
		t.Fatalf("Save #1: %v", err)
	}
	if _, _, _, err := st.Save("artist", 3, testPNG(t, 200, 201)); err != nil {
		t.Fatalf("Save #2: %v", err)
	}
	entityDir := dir + "/artist/3"

	if err := st.DeleteAll("artist", 3); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if _, err := os.Stat(entityDir); !os.IsNotExist(err) {
		t.Fatalf("entity dir %q still exists after DeleteAll", entityDir)
	}
}

func TestImageStoreDeleteAllDoesNotAffectOtherEntities(t *testing.T) {
	dir := t.TempDir()
	st := NewImageStore(dir)

	if _, _, _, err := st.Save("artist", 3, testPNG(t, 200, 200)); err != nil {
		t.Fatalf("Save artist 3: %v", err)
	}
	if _, _, _, err := st.Save("artist", 4, testPNG(t, 200, 200)); err != nil {
		t.Fatalf("Save artist 4: %v", err)
	}
	otherDir := dir + "/artist/4"

	if err := st.DeleteAll("artist", 3); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	if _, err := os.Stat(otherDir); err != nil {
		t.Fatalf("expected artist 4's artwork to survive, stat error: %v", err)
	}
}

func TestImageStoreDeleteAllOnMissingDirIsNotAnError(t *testing.T) {
	st := NewImageStore(t.TempDir())
	if err := st.DeleteAll("artist", 999); err != nil {
		t.Fatalf("DeleteAll on nonexistent dir = %v, want nil", err)
	}
}
