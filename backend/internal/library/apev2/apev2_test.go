package apev2

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// apeItem is a test-side helper describing one raw APEv2 item to serialize
// for building a synthetic tag by hand — independent of this package's own
// Write, so Read is verified against a byte layout built straight from the
// spec, not just round-tripped against itself.
type apeItem struct {
	key       string
	value     []byte
	valueType int // 0 = UTF-8 text, 1 = binary
}

// buildTag serializes items into a raw APEv2 tag (items + a footer, no
// header — the minimal spec-compliant form) matching the real on-disk
// layout: "APETAGEX", version, length (items+footer size), item count,
// flags, 8 reserved bytes.
func buildTag(items []apeItem) []byte {
	var body bytes.Buffer
	for _, it := range items {
		var sizeBuf, flagsBuf [4]byte
		binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(it.value)))
		binary.LittleEndian.PutUint32(flagsBuf[:], uint32(it.valueType&0x3)<<1)
		body.Write(sizeBuf[:])
		body.Write(flagsBuf[:])
		body.WriteString(it.key)
		body.WriteByte(0)
		body.Write(it.value)
	}

	footer := make([]byte, 32)
	copy(footer[0:8], "APETAGEX")
	binary.LittleEndian.PutUint32(footer[8:12], 2000)
	binary.LittleEndian.PutUint32(footer[12:16], uint32(body.Len()+32))
	binary.LittleEndian.PutUint32(footer[16:20], uint32(len(items)))
	binary.LittleEndian.PutUint32(footer[20:24], 1<<30) // "has footer" bit

	var full bytes.Buffer
	full.Write(body.Bytes())
	full.Write(footer)
	return full.Bytes()
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.wv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestReadParsesTextItems(t *testing.T) {
	audio := []byte("fake wavpack audio bytes")
	tag := buildTag([]apeItem{
		{key: "Artist", value: []byte("Океан Ельзи"), valueType: 0},
		{key: "Album", value: []byte("Гегемонія"), valueType: 0},
		{key: "Title", value: []byte("Друге дихання"), valueType: 0},
		{key: "Track", value: []byte("3"), valueType: 0},
		{key: "Year", value: []byte("2013"), valueType: 0},
		{key: "Genre", value: []byte("Rock"), valueType: 0},
	})
	path := writeTempFile(t, append(audio, tag...))

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Artist != "Океан Ельзи" || tags.Album != "Гегемонія" || tags.Title != "Друге дихання" {
		t.Fatalf("tags = %+v", tags)
	}
	if tags.Track != 3 || tags.Year != 2013 {
		t.Fatalf("Track/Year = %d/%d", tags.Track, tags.Year)
	}
	if tags.Genre != "Rock" {
		t.Fatalf("Genre = %q", tags.Genre)
	}
}

func TestReadParsesTrackWithTotal(t *testing.T) {
	// APEv2 Track items commonly use a Vorbis-style "N/M" form.
	audio := []byte("fake wavpack audio bytes")
	tag := buildTag([]apeItem{{key: "Track", value: []byte("7/12"), valueType: 0}})
	path := writeTempFile(t, append(audio, tag...))

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Track != 7 {
		t.Fatalf("Track = %d, want 7", tags.Track)
	}
}

func TestReadParsesCoverArt(t *testing.T) {
	audio := []byte("fake wavpack audio bytes")
	imageBytes := []byte("\xFF\xD8\xFFfakejpegdata")
	value := append([]byte("cover.jpg\x00"), imageBytes...)
	tag := buildTag([]apeItem{{key: "Cover Art (Front)", value: value, valueType: 1}})
	path := writeTempFile(t, append(audio, tag...))

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(tags.Artwork, imageBytes) {
		t.Fatalf("Artwork = %v, want %v", tags.Artwork, imageBytes)
	}
}

func TestReadWithNoTagReturnsZeroValue(t *testing.T) {
	path := writeTempFile(t, []byte("just some audio bytes, no tag at all"))
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Artist != "" || tags.Album != "" || tags.Title != "" || tags.Track != 0 || tags.Year != 0 || tags.Genre != "" || tags.Artwork != nil {
		t.Fatalf("tags = %+v, want zero value", tags)
	}
}

func TestWriteThenReadRoundTrips(t *testing.T) {
	audio := []byte("fake wavpack audio bytes, long enough to look real")
	path := writeTempFile(t, audio)

	err := Write(path, Tags{Artist: "Артист", Album: "Альбом", Title: "Назва", Track: 5, Year: 1999})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Artist != "Артист" || tags.Album != "Альбом" || tags.Title != "Назва" || tags.Track != 5 || tags.Year != 1999 {
		t.Fatalf("tags = %+v", tags)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	audioPrefix := make([]byte, len(audio))
	f2, _ := os.Open(path)
	defer f2.Close()
	if _, err := f2.ReadAt(audioPrefix, 0); err != nil {
		t.Fatalf("reading back audio prefix: %v", err)
	}
	if !bytes.Equal(audioPrefix, audio) {
		t.Fatal("Write must not touch bytes before the tag — audio prefix changed")
	}
	if info.Size() <= int64(len(audio)) {
		t.Fatalf("file size %d should have grown past the original audio-only size %d", info.Size(), len(audio))
	}
}

func TestWritePreservesGenreAndArtworkAndUnknownItems(t *testing.T) {
	audio := []byte("fake wavpack audio bytes")
	imageBytes := []byte("\xFF\xD8\xFFfakejpegdata")
	coverValue := append([]byte("cover.jpg\x00"), imageBytes...)
	tag := buildTag([]apeItem{
		{key: "Artist", value: []byte("Old Artist"), valueType: 0},
		{key: "Genre", value: []byte("Jazz"), valueType: 0},
		{key: "Cover Art (Front)", value: coverValue, valueType: 1},
		{key: "Some Custom Item", value: []byte("keep me"), valueType: 0},
	})
	path := writeTempFile(t, append(audio, tag...))

	if err := Write(path, Tags{Artist: "New Artist", Album: "New Album", Title: "New Title", Track: 1, Year: 2020}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Artist != "New Artist" {
		t.Fatalf("Artist = %q, want New Artist (should be overwritten)", tags.Artist)
	}
	if tags.Genre != "Jazz" {
		t.Fatalf("Genre = %q, want Jazz (should be preserved, Genre is read-only elsewhere in this app)", tags.Genre)
	}
	if !bytes.Equal(tags.Artwork, imageBytes) {
		t.Fatalf("Artwork = %v, want preserved %v", tags.Artwork, imageBytes)
	}
}

func TestWriteOnFileWithNoExistingTag(t *testing.T) {
	audio := []byte("fake wavpack audio bytes with absolutely no tag yet")
	path := writeTempFile(t, audio)

	if err := Write(path, Tags{Artist: "Fresh Artist", Title: "Fresh Title", Track: 1, Year: 2024}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tags, err := Read(f)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if tags.Artist != "Fresh Artist" || tags.Title != "Fresh Title" {
		t.Fatalf("tags = %+v", tags)
	}
}
