package organize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

func TestFixCyrillicMojibake(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"cyrillic mojibake", "Àçàðò", "Азарт"},
		{"empty", "", ""},
		{"plain ascii", "Test Artist", "Test Artist"},
		// Genuine ISO-8859-1 text mixes accented and plain ASCII letters —
		// must be left alone rather than reinterpreted as cp1251.
		{"genuine latin-1 text", "Café Del Mar", "Café Del Mar"},
		{"genuine latin-1 name", "Mötley Crüe", "Mötley Crüe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := fixCyrillicMojibake(c.in); got != c.want {
				t.Fatalf("fixCyrillicMojibake(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestReadTrackTagsFixesCyrillicMojibake reproduces issue #19: an old (~20
// year old) MP3 tagged by a tool that wrote Windows-1251 bytes while still
// flagging the ID3v2 frame as ISO-8859-1 (encoding byte 0). dhowden/tag
// honors that byte literally, so m.Artist() comes back as garbled
// Latin-1-supplement letters ("Àçàðò") instead of the real word ("Азарт").
func TestReadTrackTagsFixesCyrillicMojibake(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mojibake.mp3")
	copyFixture(t, filepath.Join("testdata", "untagged.mp3"), path)

	tg, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("id3v2.Open: %v", err)
	}
	tg.SetDefaultEncoding(id3v2.EncodingISO)
	// "Àçàðò" is composed entirely of Latin-1-supplement runes (U+00C0,
	// U+00E7, U+00E0, U+00F0, U+00F2); ISO-8859-1-encoding it writes exactly
	// the Windows-1251 byte sequence for "Азарт" (0xC0 0xE7 0xE0 0xF0 0xF2).
	tg.SetArtist("Àçàðò")
	tg.SetAlbum("Àçàðò")
	tg.SetTitle("Àçàðò")
	if err := tg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	tg.Close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	f.Close()

	artist, album, title, _, _, err := readTrackTags(path)
	if err != nil {
		t.Fatalf("readTrackTags: %v", err)
	}
	if artist != "Азарт" || album != "Азарт" || title != "Азарт" {
		t.Fatalf("artist/album/title = %q/%q/%q, want corrected \"Азарт\"", artist, album, title)
	}
}
