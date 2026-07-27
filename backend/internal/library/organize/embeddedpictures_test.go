package organize

import (
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacpicture/v2"
	goflac "github.com/go-flac/go-flac/v2"
)

// mp3WithPictures copies testdata/tagged.mp3 into dir and attaches an APIC
// frame for each given (pictureType, data) pair.
func mp3WithPictures(t *testing.T, dir string, pics ...id3v2.PictureFrame) string {
	t.Helper()
	dest := filepath.Join(dir, "with_pics.mp3")
	copyFixture(t, "testdata/tagged.mp3", dest)

	tg, err := id3v2.Open(dest, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("id3v2.Open: %v", err)
	}
	for _, pf := range pics {
		tg.AddAttachedPicture(pf)
	}
	if err := tg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	tg.Close()
	return dest
}

func pictureFrame(pictureType byte, mime string, data []byte) id3v2.PictureFrame {
	return id3v2.PictureFrame{Encoding: id3v2.EncodingUTF8, MimeType: mime, PictureType: pictureType, Picture: data}
}

func TestExtractMP3PicturesReturnsEveryAttachedPicture(t *testing.T) {
	dir := t.TempDir()
	front := onePixelPNG()
	back := []byte("\xFF\xD8\xFFfakejpegback")
	path := mp3WithPictures(t, dir,
		pictureFrame(id3v2.PTFrontCover, "image/png", front),
		pictureFrame(id3v2.PTBackCover, "image/jpeg", back),
	)

	pics, err := extractMP3Pictures(path)
	if err != nil {
		t.Fatalf("extractMP3Pictures: %v", err)
	}
	if len(pics) != 2 {
		t.Fatalf("len(pics) = %d, want 2", len(pics))
	}
	byType := map[string][]byte{}
	for _, p := range pics {
		byType[p.PictureType] = p.Data
	}
	if string(byType["front"]) != string(front) || string(byType["back"]) != string(back) {
		t.Fatalf("pics = %+v", pics)
	}
}

func TestExtractMP3PicturesWithNoPictures(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "no_pics.mp3")
	copyFixture(t, "testdata/tagged.mp3", dest)

	pics, err := extractMP3Pictures(dest)
	if err != nil {
		t.Fatalf("extractMP3Pictures: %v", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}

// flacWithPictures copies testdata/tagged.flac into dir and appends a
// PICTURE metadata block for each given (pictureType, mime, data) triple.
func flacWithPictures(t *testing.T, dir string, pics ...*flacpicture.MetadataBlockPicture) string {
	t.Helper()
	dest := filepath.Join(dir, "with_pics.flac")
	copyFixture(t, "testdata/tagged.flac", dest)

	f, err := goflac.ParseFile(dest)
	if err != nil {
		t.Fatalf("goflac.ParseFile: %v", err)
	}
	for _, pic := range pics {
		block := pic.Marshal()
		f.Meta = append(f.Meta, &block)
	}
	if err := f.Save(dest); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return dest
}

func newFlacPicture(t *testing.T, pictureType flacpicture.PictureType, mime string, data []byte) *flacpicture.MetadataBlockPicture {
	t.Helper()
	pic, err := flacpicture.NewFromImageData(pictureType, "", data, mime)
	if err != nil {
		t.Fatalf("NewFromImageData: %v", err)
	}
	return pic
}

func TestExtractFLACPicturesReturnsEveryPictureBlock(t *testing.T) {
	dir := t.TempDir()
	front := onePixelPNG()
	back := onePixelPNG()
	path := flacWithPictures(t, dir,
		newFlacPicture(t, flacpicture.PictureTypeFrontCover, "image/png", front),
		newFlacPicture(t, flacpicture.PictureTypeBackCover, "image/png", back),
	)

	pics, err := extractFLACPictures(path)
	if err != nil {
		t.Fatalf("extractFLACPictures: %v", err)
	}
	if len(pics) != 2 {
		t.Fatalf("len(pics) = %d, want 2", len(pics))
	}
	byType := map[string][]byte{}
	for _, p := range pics {
		byType[p.PictureType] = p.Data
	}
	if string(byType["front"]) != string(front) || string(byType["back"]) != string(back) {
		t.Fatalf("pics = %+v", pics)
	}
}

func TestExtractFLACPicturesWithNoPictures(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "no_pics.flac")
	copyFixture(t, "testdata/tagged.flac", dest)

	pics, err := extractFLACPictures(dest)
	if err != nil {
		t.Fatalf("extractFLACPictures: %v", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}

func TestExtractWavPackPicturesWithNoPictures(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "no_pics.wv")
	copyFixture(t, "testdata/tagged.wv", dest)

	pics, err := extractWavPackPictures(dest)
	if err != nil {
		t.Fatalf("extractWavPackPictures: %v", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none (fixture carries no cover art)", pics)
	}
}

func TestPictureTypeLabelKnownAndUnknown(t *testing.T) {
	cases := map[int]string{0: "other", 3: "front", 4: "back", 20: "publisher_studio_logotype"}
	for n, want := range cases {
		if got := pictureTypeLabel(n); got != want {
			t.Fatalf("pictureTypeLabel(%d) = %q, want %q", n, got, want)
		}
	}
	if got := pictureTypeLabel(99); got != "" {
		t.Fatalf("pictureTypeLabel(99) = %q, want empty", got)
	}
	if got := pictureTypeLabel(-1); got != "" {
		t.Fatalf("pictureTypeLabel(-1) = %q, want empty", got)
	}
}
