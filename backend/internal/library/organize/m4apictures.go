package organize

import (
	"encoding/binary"
	"os"
)

// mp4ContainerTypes are MP4/M4A box types whose payload is itself a
// sequence of child boxes, worth recursing into to find "covr" (cover
// art) wherever it lives in the tree (moov > udta > meta > ilst > covr).
// "meta" is a "full box": its payload carries an extra 4-byte
// version+flags header before its children that a plain container
// doesn't have.
var mp4ContainerTypes = map[string]bool{
	"moov": true, "udta": true, "meta": true, "ilst": true, "covr": true,
}

// extractM4APictures collects every `data` child of an M4A file's `covr`
// atom (TDR 014 AC-7) — hand-rolled MP4 box traversal, no existing
// dependency in this project reads M4A at all. MP4 boxes carry no
// per-picture type byte (unlike ID3v2/FLAC/APEv2), so PictureType is
// always blank for this format.
func extractM4APictures(path string) ([]EmbeddedPicture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, nil
	}

	var pics []EmbeddedPicture
	walkMP4Boxes(f, 0, info.Size(), "", false, &pics)
	return pics, nil
}

// walkMP4Boxes walks the sequence of boxes in [start, end) (the payload
// of parentType, or the whole file for the top-level call), recursing
// into containers and collecting every "data" box found under a "covr"
// ancestor. Any malformed/truncated box simply stops that branch's
// traversal rather than erroring — a partially-readable tag still yields
// whatever pictures came before the corruption.
func walkMP4Boxes(f *os.File, start, end int64, parentType string, insideCovr bool, pics *[]EmbeddedPicture) {
	pos := start
	if parentType == "meta" {
		pos += 4 // full-box version+flags
	}

	for pos+8 <= end {
		var header [8]byte
		if _, err := f.ReadAt(header[:], pos); err != nil {
			return
		}
		size := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])
		headerLen := int64(8)

		if size == 1 {
			if pos+16 > end {
				return
			}
			var ext [8]byte
			if _, err := f.ReadAt(ext[:], pos+8); err != nil {
				return
			}
			size = int64(binary.BigEndian.Uint64(ext[:]))
			headerLen = 16
		} else if size == 0 {
			size = end - pos // extends to the end of its parent
		}
		if size < headerLen || pos+size > end {
			return
		}

		payloadStart := pos + headerLen
		payloadEnd := pos + size

		if insideCovr && boxType == "data" {
			// A "data" box's own payload is a 4-byte version+type-indicator
			// header, then 4 reserved bytes, then the raw image bytes.
			const dataBoxHeaderLen = 8
			if payloadEnd-payloadStart > dataBoxHeaderLen {
				buf := make([]byte, payloadEnd-payloadStart-dataBoxHeaderLen)
				if _, err := f.ReadAt(buf, payloadStart+dataBoxHeaderLen); err == nil {
					*pics = append(*pics, EmbeddedPicture{Data: buf})
				}
			}
		}

		if mp4ContainerTypes[boxType] {
			walkMP4Boxes(f, payloadStart, payloadEnd, boxType, insideCovr || boxType == "covr", pics)
		}

		pos += size
	}
}
