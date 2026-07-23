package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// MP4 parses an MP4/M4A file's duration exactly, from the timescale and
// duration fields in its "mvhd" (movie header) atom, nested inside "moov".
// f must be positioned anywhere — MP4 only ever reads at explicit offsets.
func MP4(f *os.File) (time.Duration, error) {
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	return findMvhdDuration(f, 0, info.Size())
}

// findMvhdDuration walks sibling boxes in [start, end), recursing into
// "moov" to find "mvhd".
func findMvhdDuration(f *os.File, start, end int64) (time.Duration, error) {
	pos := start
	for pos < end {
		if _, err := f.Seek(pos, io.SeekStart); err != nil {
			return 0, err
		}
		var header [8]byte
		if _, err := io.ReadFull(f, header[:]); err != nil {
			return 0, fmt.Errorf("reading box header: %w", err)
		}
		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		headerLen := int64(8)

		if boxSize == 1 {
			var ext [8]byte
			if _, err := io.ReadFull(f, ext[:]); err != nil {
				return 0, fmt.Errorf("reading extended box size: %w", err)
			}
			boxSize = int64(binary.BigEndian.Uint64(ext[:]))
			headerLen = 16
		}
		if boxSize == 0 {
			boxSize = end - pos
		}
		if boxSize < headerLen {
			return 0, fmt.Errorf("invalid box size for %q", boxType)
		}

		switch boxType {
		case "mvhd":
			return parseMvhd(f, pos+headerLen)
		case "moov":
			if d, err := findMvhdDuration(f, pos+headerLen, pos+boxSize); err == nil {
				return d, nil
			}
		}

		pos += boxSize
	}
	return 0, fmt.Errorf("mvhd atom not found")
}

func parseMvhd(f *os.File, bodyStart int64) (time.Duration, error) {
	var versionFlags [4]byte
	if _, err := f.ReadAt(versionFlags[:], bodyStart); err != nil {
		return 0, fmt.Errorf("reading mvhd version: %w", err)
	}

	if versionFlags[0] == 1 {
		buf := make([]byte, 28)
		if _, err := f.ReadAt(buf, bodyStart+4); err != nil {
			return 0, fmt.Errorf("reading mvhd (v1) body: %w", err)
		}
		timescale := binary.BigEndian.Uint32(buf[16:20])
		dur := binary.BigEndian.Uint64(buf[20:28])
		if timescale == 0 {
			return 0, fmt.Errorf("invalid timescale")
		}
		return secondsToDuration(float64(dur), float64(timescale)), nil
	}

	buf := make([]byte, 16)
	if _, err := f.ReadAt(buf, bodyStart+4); err != nil {
		return 0, fmt.Errorf("reading mvhd (v0) body: %w", err)
	}
	timescale := binary.BigEndian.Uint32(buf[8:12])
	dur := binary.BigEndian.Uint32(buf[12:16])
	if timescale == 0 {
		return 0, fmt.Errorf("invalid timescale")
	}
	return secondsToDuration(float64(dur), float64(timescale)), nil
}
