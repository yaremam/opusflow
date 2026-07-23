package duration

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These exercise the parsers against real encoder output (fixtures shared
// with the scan package's tag-extraction tests), as a sanity check beyond
// the hand-built exact-value fixtures above: real files must parse without
// error and produce a plausible, bounded duration.
func TestDurationOnRealFixtures(t *testing.T) {
	const maxPlausible = 30 * time.Second

	tests := []struct {
		name string
		fn   func(*os.File) (time.Duration, error)
		path string
	}{
		{"mp3", MP3, filepath.Join("..", "testdata", "with_tags", "sample.mp3")},
		{"flac", FLAC, filepath.Join("..", "testdata", "with_tags", "sample.flac")},
		{"m4a", MP4, filepath.Join("..", "testdata", "with_tags", "sample.m4a")},
		{"ogg", OGG, filepath.Join("..", "testdata", "with_tags", "sample.ogg")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.fn(openTestFile(t, tt.path))
			if err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if got <= 0 || got > maxPlausible {
				t.Fatalf("%s duration = %v, want a value in (0, %v]", tt.name, got, maxPlausible)
			}
		})
	}
}
