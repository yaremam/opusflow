package duration

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// oggPage builds one minimal Ogg page: header + a single lacing segment
// containing payload.
func oggPage(granule uint64, payload []byte) []byte {
	header := make([]byte, 27)
	copy(header[0:4], "OggS")
	header[4] = 0 // version
	header[5] = 0 // header type
	binary.LittleEndian.PutUint64(header[6:14], granule)
	binary.LittleEndian.PutUint32(header[14:18], 1) // serial number
	binary.LittleEndian.PutUint32(header[18:22], 0) // page sequence number
	binary.LittleEndian.PutUint32(header[22:26], 0) // CRC (unchecked by our parser)
	header[26] = 1                                  // page_segments
	segTable := []byte{byte(len(payload))}

	buf := append([]byte{}, header...)
	buf = append(buf, segTable...)
	buf = append(buf, payload...)
	return buf
}

func vorbisIdentificationPayload(sampleRate uint32) []byte {
	p := make([]byte, 30)
	p[0] = 1
	copy(p[1:7], "vorbis")
	binary.LittleEndian.PutUint32(p[7:11], 0) // vorbis_version
	p[11] = 2                                 // channels
	binary.LittleEndian.PutUint32(p[12:16], sampleRate)
	return p
}

func writeMinimalOGG(t *testing.T, path string, sampleRate uint32, lastGranule uint64) {
	t.Helper()
	var buf []byte
	buf = append(buf, oggPage(0, vorbisIdentificationPayload(sampleRate))...)
	buf = append(buf, oggPage(lastGranule, []byte{0x00})...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOGGDuration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ogg")
	writeMinimalOGG(t, path, 48000, 48000*4) // exactly 4 seconds

	got, err := OGG(openTestFile(t, path))
	if err != nil {
		t.Fatalf("OGG: %v", err)
	}
	if got != 4*time.Second {
		t.Fatalf("OGG duration = %v, want %v", got, 4*time.Second)
	}
}

func TestOGGDurationIgnoresUnsetGranulePages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ogg")
	var buf []byte
	buf = append(buf, oggPage(0, vorbisIdentificationPayload(44100))...)
	buf = append(buf, oggPage(44100*2, []byte{0x00})...)            // 2s, the real last valid granule
	buf = append(buf, oggPage(0xFFFFFFFFFFFFFFFF, []byte{0x00})...) // "no packets finish here"
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := OGG(openTestFile(t, path))
	if err != nil {
		t.Fatalf("OGG: %v", err)
	}
	if got != 2*time.Second {
		t.Fatalf("OGG duration = %v, want %v", got, 2*time.Second)
	}
}

func TestOGGDurationRejectsNonOGG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-ogg.ogg")
	if err := os.WriteFile(path, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := OGG(openTestFile(t, path)); err == nil {
		t.Fatal("OGG(non-OGG file) error = nil, want error")
	}
}
