package duration

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeMinimalWAV(t *testing.T, path string, sampleRate uint32, channels, bitsPerSample uint16, numSamples int) {
	t.Helper()

	blockAlign := channels * (bitsPerSample / 8)
	byteRate := sampleRate * uint32(blockAlign)
	dataSize := uint32(numSamples) * uint32(blockAlign)

	buf := make([]byte, 0, 44+dataSize)
	buf = append(buf, "RIFF"...)
	buf = binary.LittleEndian.AppendUint32(buf, 36+dataSize)
	buf = append(buf, "WAVE"...)

	buf = append(buf, "fmt "...)
	buf = binary.LittleEndian.AppendUint32(buf, 16)
	buf = binary.LittleEndian.AppendUint16(buf, 1)
	buf = binary.LittleEndian.AppendUint16(buf, channels)
	buf = binary.LittleEndian.AppendUint32(buf, sampleRate)
	buf = binary.LittleEndian.AppendUint32(buf, byteRate)
	buf = binary.LittleEndian.AppendUint16(buf, blockAlign)
	buf = binary.LittleEndian.AppendUint16(buf, bitsPerSample)

	buf = append(buf, "data"...)
	buf = binary.LittleEndian.AppendUint32(buf, dataSize)
	buf = append(buf, make([]byte, dataSize)...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWAVDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.wav")
	writeMinimalWAV(t, path, 8000, 1, 16, 8000) // exactly 1 second

	got, err := WAV(openTestFile(t, path))
	if err != nil {
		t.Fatalf("WAV: %v", err)
	}
	if got != time.Second {
		t.Fatalf("WAV duration = %v, want %v", got, time.Second)
	}
}

func TestWAVDurationStereo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.wav")
	writeMinimalWAV(t, path, 44100, 2, 16, 44100*2) // 2 seconds, stereo

	got, err := WAV(openTestFile(t, path))
	if err != nil {
		t.Fatalf("WAV: %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("WAV duration = %v, want %v", got, 2*time.Second)
	}
}

func TestWAVDurationRejectsNonWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-wav.wav")
	if err := os.WriteFile(path, []byte("this is not a WAV file at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := WAV(openTestFile(t, path)); err == nil {
		t.Fatal("WAV(non-WAV file) error = nil, want error")
	}
}
