// Package scan implements the recursive filesystem walk, audio tag
// extraction, and duration parsing used to import a library directory.
package scan

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yaremam/opusflow/backend/internal/library/scan/duration"
)

// DurationParser computes an already-open audio file's playback length.
// Each function in the duration package satisfies this.
type DurationParser func(f *os.File) (time.Duration, error)

// durationParsersByExt routes a recognized audio extension straight to the
// parser that computes its duration — the single place that knows which
// extensions are supported and which parser handles each one. ".m4a" and
// ".aac" both route to duration.MP4: an ".aac" file is commonly a raw ADTS
// bitstream rather than an actual MP4 container, so this is a known
// accuracy gap, not an oversight — flagged here instead of hidden inside a
// switch, since visibility is cheap and a real ADTS parser is separate,
// larger scope.
var durationParsersByExt = map[string]DurationParser{
	".mp3":  duration.MP3,
	".flac": duration.FLAC,
	".m4a":  duration.MP4,
	".aac":  duration.MP4, // ADTS in practice; MP4 box parser is an approximation
	".ogg":  duration.OGG,
	".wav":  duration.WAV,
}

// DetectFormat reports the duration parser for a filename based on its
// extension (case-insensitive), and whether the extension was recognized
// at all.
func DetectFormat(name string) (DurationParser, bool) {
	p, ok := durationParsersByExt[strings.ToLower(filepath.Ext(name))]
	return p, ok
}
