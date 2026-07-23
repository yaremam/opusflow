package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// FLAC parses a FLAC file's duration exactly, from the sample rate and
// total sample count in its mandatory first metadata block (STREAMINFO). f
// must be positioned anywhere — FLAC seeks to the start itself before
// reading.
func FLAC(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return 0, fmt.Errorf("reading FLAC magic: %w", err)
	}
	if string(magic[:]) != "fLaC" {
		return 0, fmt.Errorf("not a FLAC file")
	}

	var blockHeader [4]byte
	if _, err := io.ReadFull(f, blockHeader[:]); err != nil {
		return 0, fmt.Errorf("reading STREAMINFO block header: %w", err)
	}
	blockType := blockHeader[0] & 0x7F
	blockLen := int(blockHeader[1])<<16 | int(blockHeader[2])<<8 | int(blockHeader[3])
	if blockType != 0 {
		return 0, fmt.Errorf("expected STREAMINFO as the first metadata block, got type %d", blockType)
	}
	if blockLen < 18 {
		return 0, fmt.Errorf("STREAMINFO block too small")
	}

	streamInfo := make([]byte, blockLen)
	if _, err := io.ReadFull(f, streamInfo); err != nil {
		return 0, fmt.Errorf("reading STREAMINFO block: %w", err)
	}

	// Bytes [10:18] pack sampleRate(20 bits) | channels-1(3) |
	// bitsPerSample-1(5) | totalSamples(36), 64 bits total, big-endian.
	packed := binary.BigEndian.Uint64(streamInfo[10:18])
	sampleRate := uint32(packed >> 44)
	totalSamples := packed & 0xFFFFFFFFF

	if sampleRate == 0 {
		return 0, fmt.Errorf("invalid sample rate")
	}

	return secondsToDuration(float64(totalSamples), float64(sampleRate)), nil
}
