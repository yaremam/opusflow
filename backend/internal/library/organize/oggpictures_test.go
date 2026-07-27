package organize

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-flac/flacpicture/v2"
)

// oggPage builds one Ogg page carrying a single packet, splitting payload
// into as many 255-byte lacing segments as needed (with a final shorter
// one — 0 if payload's length is an exact multiple of 255 — to terminate
// the packet), matching the real on-disk lacing convention.
func oggPage(headerType byte, granule uint64, payload []byte) []byte {
	var segments []byte
	i := 0
	for len(payload)-i >= 255 {
		segments = append(segments, 255)
		i += 255
	}
	segments = append(segments, byte(len(payload)-i))

	header := make([]byte, 27)
	copy(header[0:4], "OggS")
	header[5] = headerType
	binary.LittleEndian.PutUint64(header[6:14], granule)
	binary.LittleEndian.PutUint32(header[14:18], 1) // serial number
	header[26] = byte(len(segments))

	buf := append([]byte{}, header...)
	buf = append(buf, segments...)
	buf = append(buf, payload...)
	return buf
}

func le32(n int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(n))
	return b
}

// vorbisCommentPacket builds a well-formed Vorbis comment header packet
// body (packet_type + "vorbis" + vendor + comments + framing bit) out of
// plain "KEY=VALUE" comment strings.
func vorbisCommentPacket(comments ...string) []byte {
	vendor := "opusflow-test"
	var body []byte
	body = append(body, le32(len(vendor))...)
	body = append(body, vendor...)
	body = append(body, le32(len(comments))...)
	for _, c := range comments {
		body = append(body, le32(len(c))...)
		body = append(body, c...)
	}

	packet := []byte{0x03}
	packet = append(packet, "vorbis"...)
	packet = append(packet, body...)
	packet = append(packet, 0x01) // framing bit
	return packet
}

func vorbisIdentificationPacket() []byte {
	p := make([]byte, 30)
	p[0] = 1
	copy(p[1:7], "vorbis")
	binary.LittleEndian.PutUint32(p[12:16], 44100)
	return p
}

func writeOGGFile(t *testing.T, comments ...string) string {
	t.Helper()
	var buf []byte
	buf = append(buf, oggPage(0x02, 0, vorbisIdentificationPacket())...) // BOS
	buf = append(buf, oggPage(0x00, 0, vorbisCommentPacket(comments...))...)

	path := filepath.Join(t.TempDir(), "sample.ogg")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func base64PictureComment(t *testing.T, pictureType flacpicture.PictureType, data []byte) string {
	t.Helper()
	block := newFlacPicture(t, pictureType, "image/png", data).Marshal()
	return "METADATA_BLOCK_PICTURE=" + base64.StdEncoding.EncodeToString(block.Data)
}

func TestExtractOGGPicturesReturnsEmbeddedPicture(t *testing.T) {
	front := onePixelPNG()
	path := writeOGGFile(t,
		"ARTIST=Someone",
		base64PictureComment(t, flacpicture.PictureTypeFrontCover, front),
	)

	pics, err := extractOGGPictures(path)
	if err != nil {
		t.Fatalf("extractOGGPictures: %v", err)
	}
	if len(pics) != 1 {
		t.Fatalf("len(pics) = %d, want 1", len(pics))
	}
	if pics[0].PictureType != "front" || string(pics[0].Data) != string(front) {
		t.Fatalf("pics[0] = %+v", pics[0])
	}
}

func TestExtractOGGPicturesReturnsEveryPicture(t *testing.T) {
	front := onePixelPNG()
	back := onePixelPNG()
	path := writeOGGFile(t,
		base64PictureComment(t, flacpicture.PictureTypeFrontCover, front),
		base64PictureComment(t, flacpicture.PictureTypeBackCover, back),
	)

	pics, err := extractOGGPictures(path)
	if err != nil {
		t.Fatalf("extractOGGPictures: %v", err)
	}
	if len(pics) != 2 {
		t.Fatalf("len(pics) = %d, want 2", len(pics))
	}
	types := map[string]bool{}
	for _, p := range pics {
		types[p.PictureType] = true
	}
	if !types["front"] || !types["back"] {
		t.Fatalf("pics = %+v, want front and back", pics)
	}
}

func TestExtractOGGPicturesWithNoPictures(t *testing.T) {
	path := writeOGGFile(t, "ARTIST=Someone", "ALBUM=Something")

	pics, err := extractOGGPictures(path)
	if err != nil {
		t.Fatalf("extractOGGPictures: %v", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}

func TestExtractOGGPicturesRejectsNonOGG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-ogg.ogg")
	if err := os.WriteFile(path, []byte("nope, not ogg at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	pics, err := extractOGGPictures(path)
	if err != nil {
		t.Fatalf("extractOGGPictures: %v, want nil (tolerant of unreadable/malformed files)", err)
	}
	if len(pics) != 0 {
		t.Fatalf("pics = %+v, want none", pics)
	}
}
