package organize

import "testing"

func TestDestPathBuildsCanonicalLayout(t *testing.T) {
	got := destPath("/music", "Pink Floyd", "Wish You Were Here", 1975, 1, "Shine On You Crazy Diamond", ".flac")
	want := "/music/Pink Floyd/1975.Wish You Were Here/01.Shine On You Crazy Diamond.flac"
	if got != want {
		t.Fatalf("destPath = %q, want %q", got, want)
	}
}

func TestDestPathZeroPadsTrackNumber(t *testing.T) {
	got := destPath("/music", "Artist", "Album", 2000, 9, "Title", ".mp3")
	want := "/music/Artist/2000.Album/09.Title.mp3"
	if got != want {
		t.Fatalf("destPath = %q, want %q", got, want)
	}
}

func TestDestPathStripsIllegalCharacters(t *testing.T) {
	got := destPath("/music", "AC/DC", `Highway to Hell: "Live"`, 1979, 1, "Touch Too Much?", ".mp3")
	want := "/music/ACDC/1979.Highway to Hell Live/01.Touch Too Much.mp3"
	if got != want {
		t.Fatalf("destPath = %q, want %q", got, want)
	}
}

func TestDestPathLeavesBlankFieldsVisiblyBlank(t *testing.T) {
	// Missing title/year render as an incomplete-looking but non-crashing
	// path — the review screen is what surfaces this as needing input
	// (AC-7), destPath itself just has to not panic or silently invent data.
	got := destPath("/music", "Artist", "Album", 0, 4, "", ".flac")
	want := "/music/Artist/0.Album/04..flac"
	if got != want {
		t.Fatalf("destPath = %q, want %q", got, want)
	}
}
