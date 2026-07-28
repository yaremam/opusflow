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

func TestWavPackHighResSampleRates(t *testing.T) {
	// Indices 14 and 15 are the two high-res standard rates (176400 and
	// 192000) — previously mismapped by a table missing the 176400 entry,
	// which shifted 192000 into index 14 and made the real index-15 rate
	// (192000) wrongly look "non-standard". 176,400 total samples at each
	// rate is exactly 1 second, so a wrong rate would show up as a wrong
	// duration rather than just an error.
	tests := []struct {
		name string
		idx  uint32
		rate uint32
	}{
		{"176400Hz", 14, 176400},
		{"192000Hz", 15, 192000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block := wvBlock(tt.rate, 0, tt.rate, flagsForRateIndex(tt.idx), 100)
			f := writeTempWV(t, block)

			d, err := WavPack(f)
			if err != nil {
				t.Fatalf("WavPack: %v", err)
			}
			if got := d.Seconds(); got < 0.99 || got > 1.01 {
				t.Fatalf("duration = %v, want ~1s at %d Hz", d, tt.rate)
			}
		})
	}
}
