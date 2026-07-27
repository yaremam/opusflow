package organize

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"

	"github.com/go-flac/flacpicture/v2"
	goflac "github.com/go-flac/go-flac/v2"

	"github.com/go-flac/flacvorbis/v2"
)

// vorbisCommentPacketType/vorbisMagic mark an Ogg Vorbis logical
// bitstream's second packet — the comment header — per the Vorbis I
// spec: packet_type byte 3, followed by the literal string "vorbis".
const (
	vorbisCommentPacketType = 0x03
	vorbisMagic             = "vorbis"
)

// metadataBlockPictureKey is the Vorbis-comment convention (shared by Ogg
// Vorbis, Opus, and FLAC) for embedding a picture: the comment's value is
// a base64-encoded FLAC PICTURE metadata block.
const metadataBlockPictureKey = "METADATA_BLOCK_PICTURE"

// extractOGGPictures collects every METADATA_BLOCK_PICTURE comment in an
// Ogg Vorbis file's comment header (TDR 014 AC-7) — Ogg has no existing
// dependency in this project, so the container itself (pages, packets
// reassembled across page boundaries via lacing values) is hand-rolled
// here; once the comment header packet is isolated, its byte layout is
// identical to a FLAC VORBIS_COMMENT block, so flacvorbis parses it, and
// each picture value is a base64-encoded FLAC PICTURE block, so
// flacpicture parses that too — no new picture-format parsing needed.
func extractOGGPictures(path string) ([]EmbeddedPicture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil
	}
	defer f.Close()

	packets, err := readOGGPackets(f, 2)
	if err != nil || len(packets) < 2 {
		return nil, nil
	}

	comment, err := parseVorbisCommentPacket(packets[1])
	if err != nil {
		return nil, nil
	}

	values, err := comment.Get(metadataBlockPictureKey)
	if err != nil {
		return nil, nil
	}

	var pics []EmbeddedPicture
	for _, v := range values {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			continue
		}
		pic, err := flacpicture.ParseFromMetaDataBlock(goflac.MetaDataBlock{Type: goflac.Picture, Data: decoded})
		if err != nil {
			continue
		}
		pics = append(pics, EmbeddedPicture{Data: pic.ImageData, PictureType: pictureTypeLabel(int(pic.PictureType))})
	}
	return pics, nil
}

// parseVorbisCommentPacket strips packet's leading packet-type+"vorbis"
// header and trailing framing bit, leaving exactly a FLAC-style
// VORBIS_COMMENT block body — reused via flacvorbis rather than
// hand-rolling a second comment-list parser.
func parseVorbisCommentPacket(packet []byte) (*flacvorbis.MetaDataBlockVorbisComment, error) {
	prefix := len(vorbisMagic) + 1
	if len(packet) < prefix+1 || packet[0] != vorbisCommentPacketType || string(packet[1:prefix]) != vorbisMagic {
		return nil, fmt.Errorf("not a vorbis comment packet")
	}
	body := packet[prefix : len(packet)-1] // trailing byte is the framing bit
	return flacvorbis.ParseFromMetaDataBlock(goflac.MetaDataBlock{Type: goflac.VorbisComment, Data: body})
}

// readOGGPackets reassembles up to maxPackets logical packets from f's Ogg
// pages, reconstructing packets that span multiple pages via lacing
// values (a segment table entry of exactly 255 means "this packet isn't
// finished yet, more of it follows" — possibly on the next page; anything
// less terminates the packet). Assumes a single logical bitstream, same
// as this project's existing scan/duration.OGG — good enough for an
// audio file that's just one Vorbis stream, which is all this app scans.
func readOGGPackets(f *os.File, maxPackets int) ([][]byte, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var packets [][]byte
	var pending []byte

	for len(packets) < maxPackets {
		var header [27]byte
		if _, err := io.ReadFull(f, header[:]); err != nil {
			break
		}
		if string(header[0:4]) != "OggS" {
			return nil, fmt.Errorf("not an OGG file")
		}

		pageSegments := int(header[26])
		segTable := make([]byte, pageSegments)
		if _, err := io.ReadFull(f, segTable); err != nil {
			return nil, fmt.Errorf("reading segment table: %w", err)
		}

		for _, segLen := range segTable {
			buf := make([]byte, segLen)
			if _, err := io.ReadFull(f, buf); err != nil {
				return nil, fmt.Errorf("reading page payload: %w", err)
			}
			pending = append(pending, buf...)
			if segLen < 255 {
				packets = append(packets, pending)
				pending = nil
				if len(packets) >= maxPackets {
					break
				}
			}
		}
	}
	return packets, nil
}
