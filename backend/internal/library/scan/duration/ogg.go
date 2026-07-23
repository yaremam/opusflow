package duration

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"
)

// noPacketsGranule is the reserved granule position value (-1 as an
// unsigned 64-bit int) meaning "no packet finishes on this page."
const noPacketsGranule = 0xFFFFFFFFFFFFFFFF

// OGG estimates an Ogg Vorbis file's duration from the sample rate in its
// first page's Vorbis identification header, and the granule position
// (sample count) of its last page that actually completes a packet. f must
// be positioned anywhere — OGG seeks to the start itself before reading.
func OGG(f *os.File) (time.Duration, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var sampleRate uint32
	var lastGranule uint64
	haveGranule := false
	firstPage := true

	for {
		var fixed [27]byte
		if _, err := io.ReadFull(f, fixed[:]); err != nil {
			if err == io.EOF {
				break
			}
			if firstPage {
				return 0, fmt.Errorf("reading first page header: %w", err)
			}
			break
		}
		if string(fixed[0:4]) != "OggS" {
			return 0, fmt.Errorf("not an OGG file")
		}

		granule := binary.LittleEndian.Uint64(fixed[6:14])
		pageSegments := int(fixed[26])

		segTable := make([]byte, pageSegments)
		if _, err := io.ReadFull(f, segTable); err != nil {
			return 0, fmt.Errorf("reading segment table: %w", err)
		}
		payloadSize := 0
		for _, s := range segTable {
			payloadSize += int(s)
		}

		payload := make([]byte, payloadSize)
		if _, err := io.ReadFull(f, payload); err != nil {
			return 0, fmt.Errorf("reading page payload: %w", err)
		}

		if firstPage {
			if payloadSize >= 16 && string(payload[1:7]) == "vorbis" {
				sampleRate = binary.LittleEndian.Uint32(payload[12:16])
			}
			firstPage = false
		}

		if granule != noPacketsGranule {
			lastGranule = granule
			haveGranule = true
		}
	}

	if sampleRate == 0 {
		return 0, fmt.Errorf("vorbis identification header not found")
	}
	if !haveGranule {
		return 0, fmt.Errorf("no page with a valid granule position found")
	}

	return secondsToDuration(float64(lastGranule), float64(sampleRate)), nil
}
