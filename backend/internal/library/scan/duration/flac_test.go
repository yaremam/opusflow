package duration

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeMinimalFLAC writes a "fLaC" magic + a single STREAMINFO metadata
// block encoding the given sample rate and total sample count. No audio
// frames are included — Duration only needs STREAMINFO.
func writeMinimalFLAC(t *testing.T, path string, sampleRate uint32, channels, bitsPerSample uint8, totalSamples uint64) {
	t.Helper()

	streamInfo := make([]byte, 34)
	// bytes[0:2] min block size, [2:4] max block size, [4:7] min frame size,
	// [7:10] max frame size — irrelevant to duration, left zero.

	// bytes[10:18]: sampleRate(20) | channels-1(3) | bitsPerSample-1(5) | totalSamples(36), packed big-endian.
	var packed uint64
	packed |= uint64(sampleRate&0xFFFFF) << 44
	packed |= uint64((channels-1)&0x7) << 41
	packed |= uint64((bitsPerSample-1)&0x1F) << 36
	packed |= totalSamples & 0xFFFFFFFFF
	binary.BigEndian.PutUint64(streamInfo[10:18], packed)

	var buf []byte
	buf = append(buf, "fLaC"...)

	blockHeader := make([]byte, 4)
	blockHeader[0] = 0x80 // is-last bit set, block type 0 = STREAMINFO
	blockHeader[1] = byte(len(streamInfo) >> 16)
	blockHeader[2] = byte(len(streamInfo) >> 8)
	blockHeader[3] = byte(len(streamInfo))
	buf = append(buf, blockHeader...)
	buf = append(buf, streamInfo...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFLACDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.flac")
	writeMinimalFLAC(t, path, 44100, 2, 16, 44100*3) // exactly 3 seconds

	got, err := FLAC(openTestFile(t, path))
	if err != nil {
		t.Fatalf("FLAC: %v", err)
	}
	if got != 3*time.Second {
		t.Fatalf("FLAC duration = %v, want %v", got, 3*time.Second)
	}
}

func TestFLACDurationFractionalSeconds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.flac")
	writeMinimalFLAC(t, path, 48000, 2, 16, 24000) // 0.5 seconds

	got, err := FLAC(openTestFile(t, path))
	if err != nil {
		t.Fatalf("FLAC: %v", err)
	}
	if got != 500*time.Millisecond {
		t.Fatalf("FLAC duration = %v, want %v", got, 500*time.Millisecond)
	}
}

func TestFLACDurationRejectsNonFLAC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-flac.flac")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := FLAC(openTestFile(t, path)); err == nil {
		t.Fatal("FLAC(non-FLAC file) error = nil, want error")
	}
}
