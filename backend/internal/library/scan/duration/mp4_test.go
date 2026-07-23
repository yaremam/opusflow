package duration

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func box(boxType string, body []byte) []byte {
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b[0:4], uint32(8+len(body)))
	copy(b[4:8], boxType)
	return append(b, body...)
}

func mvhdV0(timescale, dur uint32) []byte {
	body := make([]byte, 4+16)                // version+flags, then creation/modification/timescale/duration
	binary.BigEndian.PutUint32(body[4:8], 0)  // creation_time
	binary.BigEndian.PutUint32(body[8:12], 0) // modification_time
	binary.BigEndian.PutUint32(body[12:16], timescale)
	binary.BigEndian.PutUint32(body[16:20], dur)
	return box("mvhd", body)
}

func mvhdV1(timescale uint32, dur uint64) []byte {
	body := make([]byte, 4+28)
	body[0] = 1                                // version
	binary.BigEndian.PutUint64(body[4:12], 0)  // creation_time
	binary.BigEndian.PutUint64(body[12:20], 0) // modification_time
	binary.BigEndian.PutUint32(body[20:24], timescale)
	binary.BigEndian.PutUint64(body[24:32], dur)
	return box("mvhd", body)
}

func writeMP4(t *testing.T, path string, mvhd []byte) {
	t.Helper()
	ftyp := box("ftyp", append([]byte("isom"), 0, 0, 0, 0))
	moov := box("moov", mvhd)
	buf := append(append([]byte{}, ftyp...), moov...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMP4DurationVersion0(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.m4a")
	writeMP4(t, path, mvhdV0(1000, 5000)) // 5.0s

	got, err := MP4(openTestFile(t, path))
	if err != nil {
		t.Fatalf("MP4: %v", err)
	}
	if got != 5*time.Second {
		t.Fatalf("MP4 duration = %v, want %v", got, 5*time.Second)
	}
}

func TestMP4DurationVersion1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.m4a")
	writeMP4(t, path, mvhdV1(48000, 240000)) // 5.0s

	got, err := MP4(openTestFile(t, path))
	if err != nil {
		t.Fatalf("MP4: %v", err)
	}
	if got != 5*time.Second {
		t.Fatalf("MP4 duration = %v, want %v", got, 5*time.Second)
	}
}

func TestMP4DurationMissingMvhd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.m4a")
	ftyp := box("ftyp", append([]byte("isom"), 0, 0, 0, 0))
	moov := box("moov", box("trak", []byte{}))
	buf := append(append([]byte{}, ftyp...), moov...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := MP4(openTestFile(t, path)); err == nil {
		t.Fatal("MP4(no mvhd) error = nil, want error")
	}
}
