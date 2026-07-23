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
			want := Tags{
				Title:       "Test Title",
				Artist:      "Test Artist",
				Album:       "Test Album",
				TrackNumber: 3,
				Year:        2000,
				Genre:       "Jazz",
			}
			if tags != want {
				t.Fatalf("ExtractTags(%s) = %+v, want %+v", name, tags, want)
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
