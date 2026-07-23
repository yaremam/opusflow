package scan

import (
	"reflect"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/scan/duration"
)

// parserIdentity returns a comparable identity for a DurationParser, since
// func values in Go aren't comparable with ==.
func parserIdentity(p DurationParser) uintptr {
	return reflect.ValueOf(p).Pointer()
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name       string
		wantParser DurationParser
		wantOK     bool
	}{
		{"song.mp3", duration.MP3, true},
		{"song.MP3", duration.MP3, true},
		{"song.flac", duration.FLAC, true},
		{"song.m4a", duration.MP4, true},
		{"song.aac", duration.MP4, true},
		{"song.ogg", duration.OGG, true},
		{"song.wav", duration.WAV, true},
		{"cover.jpg", nil, false},
		{"README.md", nil, false},
		{"noextension", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := DetectFormat(tt.name)
			if ok != tt.wantOK {
				t.Fatalf("DetectFormat(%q) ok = %v, want %v", tt.name, ok, tt.wantOK)
			}
			if ok && parserIdentity(got) != parserIdentity(tt.wantParser) {
				t.Fatalf("DetectFormat(%q) returned a different parser than expected", tt.name)
			}
		})
	}
}
