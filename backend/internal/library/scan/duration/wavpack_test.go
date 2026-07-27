package duration

import (
	"encoding/binary"
	"os"
	"testing"
)

// wvBlock builds one WavPack block: the 32-byte header (matching the real
// on-disk WavpackHeader struct) followed by fillerBytes of padding, so the
// declared ckSize (block size, minus the ckID+ckSize fields themselves)
// matches what a reader following the header would actually skip over.
func wvBlock(totalSamples, blockIndex, blockSamples, flags uint32, fillerBytes int) []byte {
	const headerSize = 32
	ckSize := uint32(headerSize - 8 + fillerBytes)

	b := make([]byte, headerSize+fillerBytes)
	copy(b[0:4], "wvpk")
	binary.LittleEndian.PutUint32(b[4:8], ckSize)
	binary.LittleEndian.PutUint16(b[8:10], 0x0410) // version
	b[10] = 0                                      // track_no
	b[11] = 0                                      // index_no
	binary.LittleEndian.PutUint32(b[12:16], totalSamples)
	binary.LittleEndian.PutUint32(b[16:20], blockIndex)
	binary.LittleEndian.PutUint32(b[20:24], blockSamples)
	binary.LittleEndian.PutUint32(b[24:28], flags)
	binary.LittleEndian.PutUint32(b[28:32], 0) // crc, unchecked by this parser
	return b
}

// flagsForRateIndex sets only the sample-rate index (bits 23-26) — the one
// field WavPack's duration path needs from flags.
func flagsForRateIndex(idx uint32) uint32 {
	return (idx & 0xF) << 23
}

func writeTempWV(t *testing.T, data []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test-*.wv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestWavPackKnownTotalSamples(t *testing.T) {
	// 44100 Hz (rate index 9 in the standard table), 88200 total samples = 2s.
	block := wvBlock(88200, 0, 88200, flagsForRateIndex(9), 100)
	f := writeTempWV(t, block)

	d, err := WavPack(f)
	if err != nil {
		t.Fatalf("WavPack: %v", err)
	}
	if got := d.Seconds(); got < 1.99 || got > 2.01 {
		t.Fatalf("duration = %v, want ~2s", d)
	}
}

func TestWavPackUnknownTotalSamplesScansBlocks(t *testing.T) {
	rate := flagsForRateIndex(9) // 44100 Hz
	var data []byte
	data = append(data, wvBlock(0xFFFFFFFF, 0, 44100, rate, 50)...)     // 1s
	data = append(data, wvBlock(0xFFFFFFFF, 44100, 44100, rate, 50)...) // +1s
	data = append(data, wvBlock(0xFFFFFFFF, 88200, 22050, rate, 50)...) // +0.5s
	f := writeTempWV(t, data)

	d, err := WavPack(f)
	if err != nil {
		t.Fatalf("WavPack: %v", err)
	}
	if got := d.Seconds(); got < 2.49 || got > 2.51 {
		t.Fatalf("duration = %v, want ~2.5s (summed across blocks)", d)
	}
}

func TestWavPackRejectsWrongMagic(t *testing.T) {
	data := make([]byte, 32)
	copy(data[0:4], "nope")
	f := writeTempWV(t, data)

	if _, err := WavPack(f); err == nil {
		t.Fatal("expected an error for a non-WavPack file")
	}
}

func TestWavPackNonStandardSampleRateIsAnError(t *testing.T) {
	// Rate index 15 (0xF) means "not one of the standard rates" — this
	// hand-rolled parser doesn't chase the decoder-config extension block
	// that would carry the real rate, so it errors rather than guessing.
	block := wvBlock(88200, 0, 88200, flagsForRateIndex(15), 100)
	f := writeTempWV(t, block)

	if _, err := WavPack(f); err == nil {
		t.Fatal("expected an error for a non-standard sample rate index")
	}
}
