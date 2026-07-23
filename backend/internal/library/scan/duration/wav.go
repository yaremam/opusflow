package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// WAV parses a PCM WAV file's duration exactly, from its "fmt " and "data"
// chunk sizes. f must be positioned anywhere — WAV seeks to the start
// itself before reading.
func WAV(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var riffHeader [12]byte
	if _, err := io.ReadFull(f, riffHeader[:]); err != nil {
		return 0, fmt.Errorf("reading RIFF header: %w", err)
	}
	if string(riffHeader[0:4]) != "RIFF" || string(riffHeader[8:12]) != "WAVE" {
		return 0, fmt.Errorf("not a RIFF/WAVE file")
	}

	var byteRate uint32
	var dataSize uint32
	haveFmt, haveData := false, false

	for !(haveFmt && haveData) {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(f, chunkHeader[:]); err != nil {
			break
		}
		id := string(chunkHeader[0:4])
		size := binary.LittleEndian.Uint32(chunkHeader[4:8])
		padded := int64(size) + int64(size%2)

		switch id {
		case "fmt ":
			body := make([]byte, size)
			if _, err := io.ReadFull(f, body); err != nil {
				return 0, fmt.Errorf("reading fmt chunk: %w", err)
			}
			if size < 16 {
				return 0, fmt.Errorf("fmt chunk too small")
			}
			channels := binary.LittleEndian.Uint16(body[2:4])
			sampleRate := binary.LittleEndian.Uint32(body[4:8])
			bitsPerSample := binary.LittleEndian.Uint16(body[14:16])
			blockAlign := channels * (bitsPerSample / 8)
			byteRate = sampleRate * uint32(blockAlign)
			haveFmt = true
			if pad := padded - int64(size); pad > 0 {
				f.Seek(pad, io.SeekCurrent)
			}
		case "data":
			dataSize = size
			haveData = true
			// Don't bother reading the audio payload itself.
			f.Seek(padded, io.SeekCurrent)
		default:
			f.Seek(padded, io.SeekCurrent)
		}
	}

	if !haveFmt || !haveData {
		return 0, fmt.Errorf("missing fmt or data chunk")
	}
	if byteRate == 0 {
		return 0, fmt.Errorf("invalid byte rate (zero sample rate or block align)")
	}

	return secondsToDuration(float64(dataSize), float64(byteRate)), nil
}
