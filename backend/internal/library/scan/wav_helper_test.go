package scan

import (
	"encoding/binary"
	"os"
	"testing"
)

type wavParams struct {
	sampleRate    uint32
	channels      uint16
	bitsPerSample uint16
	numSamples    int
}

// writeMinimalWAV writes a bare-bones PCM WAV file (silence) to path, just
// enough to be a structurally valid RIFF/WAVE file for tag/duration parsing
// tests.
func writeMinimalWAV(t *testing.T, path string, p wavParams) {
	t.Helper()

	blockAlign := p.channels * (p.bitsPerSample / 8)
	byteRate := p.sampleRate * uint32(blockAlign)
	dataSize := uint32(p.numSamples) * uint32(blockAlign)

	buf := make([]byte, 0, 44+dataSize)
	buf = append(buf, "RIFF"...)
	buf = binary.LittleEndian.AppendUint32(buf, 36+dataSize)
	buf = append(buf, "WAVE"...)

	buf = append(buf, "fmt "...)
	buf = binary.LittleEndian.AppendUint32(buf, 16)
	buf = binary.LittleEndian.AppendUint16(buf, 1) // PCM
	buf = binary.LittleEndian.AppendUint16(buf, p.channels)
	buf = binary.LittleEndian.AppendUint32(buf, p.sampleRate)
	buf = binary.LittleEndian.AppendUint32(buf, byteRate)
	buf = binary.LittleEndian.AppendUint16(buf, blockAlign)
	buf = binary.LittleEndian.AppendUint16(buf, p.bitsPerSample)

	buf = append(buf, "data"...)
	buf = binary.LittleEndian.AppendUint32(buf, dataSize)
	buf = append(buf, make([]byte, dataSize)...)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}
