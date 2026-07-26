package scan

import (
	"os"
	"path/filepath"
	"testing"
)

// openTestFile opens path for reading, closing it automatically when the
// test ends — ExtractTags takes an already-open file since the scanner
// shares one file handle between tag and duration extraction.
func openTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestExtractTagsFromTaggedFiles(t *testing.T) {
	for _, name := range []string{"sample.mp3", "sample.flac", "sample.m4a", "sample.ogg"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "with_tags", name)
			tags, err := ExtractTags(path, openTestFile(t, path))
			if err != nil {
				t.Fatalf("ExtractTags: %v", err)
			}
			// Compared field-by-field rather than via struct equality —
			// Tags carries a []byte (Artwork) since AC-1, and a struct
			// with a slice field is never comparable with ==/!=, even
			// when that slice is nil. These fixtures carry no embedded
			// art, so Artwork is asserted nil alongside the rest.
			want := Tags{
				Title:       "Test Title",
				Artist:      "Test Artist",
				Album:       "Test Album",
				TrackNumber: 3,
				Year:        2000,
				Genre:       "Jazz",
			}
			if tags.Title != want.Title || tags.Artist != want.Artist || tags.Album != want.Album ||
				tags.TrackNumber != want.TrackNumber || tags.Year != want.Year || tags.Genre != want.Genre {
				t.Fatalf("ExtractTags(%s) = %+v, want %+v", name, tags, want)
			}
			if tags.Artwork != nil {
				t.Fatalf("ExtractTags(%s).Artwork = %v, want nil (fixture carries no embedded art)", name, tags.Artwork)
			}
		})
	}
}

func TestExtractTagsFallsBackToFilenameWhenUntagged(t *testing.T) {
	// Covers two distinct dhowden/tag behaviors that must both resolve to
	// the same fallback: an MP3 with no ID3 data at all returns
	// tag.ErrNoTagsFound, while an untagged FLAC/M4A/OGG container is
	// still recognized but its fields all come back empty.
	for _, name := range []string{"sample.mp3", "sample.flac", "sample.m4a", "sample.ogg"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("testdata", "without_tags", name)
			tags, err := ExtractTags(path, openTestFile(t, path))
			if err != nil {
				t.Fatalf("ExtractTags: %v", err)
			}
			if tags.Title != "sample" {
				t.Fatalf("Title = %q, want fallback %q", tags.Title, "sample")
			}
			if tags.Artist != "" || tags.Album != "" || tags.TrackNumber != 0 || tags.Year != 0 || tags.Genre != "" {
				t.Fatalf("expected zero-value fields alongside the fallback title, got %+v", tags)
			}
		})
	}
}

func TestExtractTagsFallsBackForWAV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Some Song.wav")
	writeMinimalWAV(t, path, wavParams{sampleRate: 8000, channels: 1, bitsPerSample: 8, numSamples: 800})

	tags, err := ExtractTags(path, openTestFile(t, path))
	if err != nil {
		t.Fatalf("ExtractTags: %v", err)
	}
	if tags.Title != "Some Song" {
		t.Fatalf("Title = %q, want %q (WAV rarely carries tags, so filename is the fallback)", tags.Title, "Some Song")
	}
}

// syncSafe encodes n as a 4-byte ID3v2 syncsafe integer (7 usable bits per
// byte), used for the tag header's total-size field.
func syncSafe(n uint32) [4]byte {
	return [4]byte{
		byte((n >> 21) & 0x7F),
		byte((n >> 14) & 0x7F),
		byte((n >> 7) & 0x7F),
		byte(n & 0x7F),
	}
}

// writeMinimalMP3WithAPIC writes an ID3v2.3 tag containing a single APIC
// (embedded picture) frame, with imageData as its picture bytes. There's no
// real audio data after the tag — dhowden/tag's ID3v2 reader only consumes
// the declared tag region, so this is sufficient for exercising tag/artwork
// extraction without needing a full valid MP3 stream.
func writeMinimalMP3WithAPIC(t *testing.T, path string, imageData []byte) {
	t.Helper()

	const mimeType = "image/png"
	var frameContent []byte
	frameContent = append(frameContent, 0x00)         // text encoding: ISO-8859-1
	frameContent = append(frameContent, mimeType...)  // MIME type
	frameContent = append(frameContent, 0x00)         // MIME type terminator
	frameContent = append(frameContent, 0x03)         // picture type: cover (front)
	frameContent = append(frameContent, 0x00)         // empty description + terminator
	frameContent = append(frameContent, imageData...) // picture data

	var frame []byte
	frame = append(frame, "APIC"...)
	frameSize := uint32(len(frameContent))
	frame = append(frame, byte(frameSize>>24), byte(frameSize>>16), byte(frameSize>>8), byte(frameSize)) // regular (non-syncsafe) size, per ID3v2.3
	frame = append(frame, 0x00, 0x00)                                                                    // flags
	frame = append(frame, frameContent...)

	tagSize := syncSafe(uint32(len(frame)))
	var b []byte
	b = append(b, 'I', 'D', '3', 3, 0, 0) // "ID3", version 2.3, flags
	b = append(b, tagSize[:]...)
	b = append(b, frame...)

	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExtractTagsReturnsEmbeddedArtwork(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "with_art.mp3")
	imageData := []byte("pretend-this-is-png-bytes")
	writeMinimalMP3WithAPIC(t, path, imageData)

	tags, err := ExtractTags(path, openTestFile(t, path))
	if err != nil {
		t.Fatalf("ExtractTags: %v", err)
	}
	if string(tags.Artwork) != string(imageData) {
		t.Fatalf("Artwork = %q, want %q", tags.Artwork, imageData)
	}
}

func TestExtractTagsErrorsOnCorruptTag(t *testing.T) {
	// A file that announces an ID3v2 tag but truncates the declared frame
	// data must surface as a genuine error, not the soft "no tags" fallback.
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.mp3")

	var b []byte
	b = append(b, 'I', 'D', '3', 3, 0, 0)
	// Syncsafe size claiming far more tag data than the file actually has.
	b = append(b, 0x7F, 0x7F, 0x7F, 0x7F)
	b = append(b, []byte("TIT2")...) // frame ID, then a bogus/truncated frame
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ExtractTags(path, openTestFile(t, path))
	if err == nil {
		t.Fatal("ExtractTags(corrupt tag) error = nil, want error")
	}
}
