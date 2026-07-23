package duration

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mpeg1LayerIIIHeader builds a 4-byte MPEG1 Layer III frame header:
// bitrateIndex per the standard table (9 => 128kbps), sampleRateIndex
// (0=44100, 1=48000, 2=32000), stereo channel mode.
func mpeg1LayerIIIHeader(bitrateIndex, sampleRateIndex byte) []byte {
	// byte1 bits: sync-continued(111) version=MPEG1(11) layer=III(01) protection(1) = 1111 1011 = 0xFB.
	byte1 := byte(0xFB)
	byte2 := (bitrateIndex << 4) | (sampleRateIndex << 2) // padding=0, private=0
	byte3 := byte(0x00)                                   // stereo, no emphasis
	return []byte{0xFF, byte1, byte2, byte3}
}

func TestMP3DurationCBREstimate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.mp3")

	header := mpeg1LayerIIIHeader(9, 0) // 128kbps, 44100Hz
	sideInfo := make([]byte, 32)        // MPEG1 stereo side info size

	const wantSeconds = 1.0
	const bitrateBps = 128000
	audioBytes := int(bitrateBps * wantSeconds / 8) // 16000 bytes
	filler := make([]byte, audioBytes-len(header)-len(sideInfo))

	buf := append([]byte{}, header...)
	buf = append(buf, sideInfo...)
	buf = append(buf, filler...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := MP3(openTestFile(t, path))
	if err != nil {
		t.Fatalf("MP3: %v", err)
	}
	if got != time.Second {
		t.Fatalf("MP3 duration = %v, want %v", got, time.Second)
	}
}

func TestMP3DurationXingVBR(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.mp3")

	header := mpeg1LayerIIIHeader(9, 1) // bitrate irrelevant here; 48000Hz
	sideInfo := make([]byte, 32)

	xing := make([]byte, 12)
	copy(xing[0:4], "Xing")
	binary.BigEndian.PutUint32(xing[4:8], 1)    // flags: frames field present
	binary.BigEndian.PutUint32(xing[8:12], 125) // frame count

	buf := append([]byte{}, header...)
	buf = append(buf, sideInfo...)
	buf = append(buf, xing...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := MP3(openTestFile(t, path))
	if err != nil {
		t.Fatalf("MP3: %v", err)
	}
	// 125 frames * 1152 samples/frame (MPEG1 Layer III) / 48000Hz = 3.0s exactly.
	if got != 3*time.Second {
		t.Fatalf("MP3 duration = %v, want %v", got, 3*time.Second)
	}
}

func TestMP3DurationSkipsID3v2Header(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.mp3")

	id3 := make([]byte, 10)
	copy(id3[0:3], "ID3")
	id3[3] = 3 // version
	// syncsafe size = 20 bytes of tag data follows the 10-byte header
	id3[6], id3[7], id3[8], id3[9] = 0, 0, 0, 20
	tagData := make([]byte, 20)

	header := mpeg1LayerIIIHeader(9, 0)
	sideInfo := make([]byte, 32)
	const bitrateBps = 128000
	audioBytes := int(bitrateBps * 1.0 / 8)
	filler := make([]byte, audioBytes-len(header)-len(sideInfo))

	buf := append([]byte{}, id3...)
	buf = append(buf, tagData...)
	buf = append(buf, header...)
	buf = append(buf, sideInfo...)
	buf = append(buf, filler...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := MP3(openTestFile(t, path))
	if err != nil {
		t.Fatalf("MP3: %v", err)
	}
	if got != time.Second {
		t.Fatalf("MP3 duration = %v, want %v", got, time.Second)
	}
}

func TestMP3DurationRejectsNonMP3(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-mp3.mp3")
	if err := os.WriteFile(path, []byte("definitely not mpeg audio data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := MP3(openTestFile(t, path)); err == nil {
		t.Fatal("MP3(non-MP3 file) error = nil, want error")
	}
}
