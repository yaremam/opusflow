package organize

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// mp4Box wraps payload in a standard [size][type] box header.
func mp4Box(boxType string, payload []byte) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], uint32(8+len(payload)))
	copy(buf[4:8], boxType)
	return append(buf, payload...)
}

// mp4DataBox wraps imageData as a "data" box's payload: a 4-byte
// version+type-indicator header, 4 reserved bytes, then the raw bytes.
func mp4DataBox(imageData []byte) []byte {
	payload := make([]byte, 8+len(imageData))
	payload[3] = 13 // type indicator: JPEG (not read by this app's parser)
	copy(payload[8:], imageData)
	return mp4Box("data", payload)
}

// writeM4AWithCovers builds a minimal moov > udta > meta > ilst > covr
// tree carrying one "data" box per picture, and writes it as a standalone
// file — this app's parser walks from file offset 0, so a real "ftyp" box
// isn't needed for the traversal itself to find "covr".
func writeM4AWithCovers(t *testing.T, pictures ...[]byte) string {
	t.Helper()
	var covrChildren []byte
	for _, pic := range pictures {
		covrChildren = append(covrChildren, mp4DataBox(pic)...)
	}
	covr := mp4Box("covr", covrChildren)
	ilst := mp4Box("ilst", covr)
	meta := mp4Box("meta", append([]byte{0, 0, 0, 0}, ilst...))
	udta := mp4Box("udta", meta)
	moov := mp4Box("moov", udta)

	path := filepath.Join(t.TempDir(), "sample.m4a")
	if err := os.WriteFile(path, moov, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractM4APicturesReturnsEveryDataChild(t *testing.T) {
	front := onePixelPNG()
	back := []byte("\xFF\xD8\xFFfakejpegback")
	path := writeM4AWithCovers(t, front, back)

	pics, err := extractM4APictures(path)
	if err != nil {
		t.Fatalf("extractM4APictures: %v", err)
	}
	if len(pics) != 2 {
		t.Fatalf("len(pics) = %d, want 2", len(pics))
	}
	if string(pics[0].Data) != string(front) || string(pics[1].Data) != string(back) {
		t.Fatalf("pics = %+v", pics)
	}
	for _, p := range pics {
		if p.PictureType != "" {
			t.Fatalf("PictureType = %q, want blank (MP4 has no per-picture type byte)", p.PictureType)
		}
	}
}

func TestExtractM4APicturesWithNoCovr(t *testing.T) {
	// A meta/ilst tree with no covr atom at all — e.g. a track with text
	// metadata (title, artist) but no embedded artwork.
	other := mp4Box("data", append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, "Some Title"...))
	item := mp4Box("\xa9nam", other)
	ilst := mp4Box("ilst", item)
	meta := mp4Box("meta", append([]byte{0, 0, 0, 0}, ilst...))
	udta := mp4Box("udta", meta)
	moov := mp4Box("moov", udta)

	path := filepath.Join(t.TempDir(), "no_covr.m4a")
	if err := os.WriteFile(path, moov, 0o644); err != nil {
		t.Fatal(err)
	}

	pics, err := extractM4APictures(path)
	if err != nil {
		t.Fatalf("extractM4APictures: %v", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}

func TestExtractM4APicturesRejectsGarbageFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garbage.m4a")
	if err := os.WriteFile(path, []byte("not an mp4 file at all, just text"), 0o644); err != nil {
		t.Fatal(err)
	}

	pics, err := extractM4APictures(path)
	if err != nil {
		t.Fatalf("extractM4APictures: %v, want nil (tolerant of unreadable/malformed files)", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}
