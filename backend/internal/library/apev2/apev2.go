// Package apev2 reads and writes APEv2 tags — the tag format WavPack (.wv)
// files use, distinct from the ID3v2/Vorbis-comment formats
// github.com/dhowden/tag already handles for every other format this
// project imports (TDR 013). A leaf package with no knowledge of Plan/
// Track, matching the shape of scan/duration (format-specific parsing,
// nothing else).
package apev2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// footerSize is the fixed size of an APEv2 header/footer (preamble,
// version, length, item count, flags, 8 reserved bytes).
const footerSize = 32

const preamble = "APETAGEX"

// Value types packed into bits 1-2 of an item's flags.
const (
	valueTypeUTF8   = 0
	valueTypeBinary = 1
)

// Tags is what this package reads and writes: the fields opusflow's review
// screen edits (Artist/Album/Title/Track/Year) plus Genre — read-only
// everywhere in this app for every format, WavPack included (Write
// preserves whatever was already there, the same only-touch-the-fields-
// given approach organize.setVorbisComment already takes for FLAC).
// Embedded artwork isn't part of Tags — see ReadArtworks, which returns
// every "Cover Art (*)" item rather than just one (TDR 014 AC-7).
type Tags struct {
	Artist string
	Album  string
	Title  string
	Track  int
	Year   int
	Genre  string
}

// Artwork is one embedded picture read from an APEv2 tag's "Cover Art
// (*)" items — WavPack's format supports several, distinctly keyed
// ("Cover Art (Front)", "Cover Art (Back)", "Cover Art (Booklet)", ...).
type Artwork struct {
	Data        []byte
	PictureType string // "front", "back", "booklet", ... parsed from the item's own key; "" if the key doesn't carry a recognizable type
}

// coverArtKeyPrefix is the APEv2 item-key convention every "Cover Art"
// item shares: "Cover Art (<Type>)".
const coverArtKeyPrefix = "cover art ("

// artworkPictureType extracts and lowercases <Type> from a lowercased
// "cover art (<type>)" item key, "" if key doesn't have a closing paren.
func artworkPictureType(lowerKey string) string {
	if !strings.HasSuffix(lowerKey, ")") {
		return ""
	}
	return lowerKey[len(coverArtKeyPrefix) : len(lowerKey)-1]
}

// parseArtworkValue splits a Cover Art item's value into just the image
// bytes — APEv2 packs an optional filename before a NUL, then the raw
// image data.
func parseArtworkValue(value []byte) []byte {
	if idx := bytes.IndexByte(value, 0); idx >= 0 {
		return value[idx+1:]
	}
	return value
}

// ReadArtworks returns every "Cover Art (*)" item in path's APEv2 tag —
// a WavPack file can carry several (front, back, booklet, ...), all
// preserved untouched by Write the same as any other item this package
// doesn't know about.
func ReadArtworks(path string) ([]Artwork, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	items, _, err := readItems(f)
	if err != nil {
		return nil, err
	}

	var artworks []Artwork
	for _, it := range items {
		lower := strings.ToLower(it.key)
		if !strings.HasPrefix(lower, coverArtKeyPrefix) {
			continue
		}
		artworks = append(artworks, Artwork{Data: parseArtworkValue(it.value), PictureType: artworkPictureType(lower)})
	}
	return artworks, nil
}

type item struct {
	key       string
	value     []byte
	valueType int
}

// Read parses whatever APEv2 tag is present at the end of f, returning a
// zero Tags with no error if there isn't one — the same
// not-an-error-just-empty convention github.com/dhowden/tag's
// ErrNoTagsFound already gets treated as by organize.readTrackTags/
// readGenreAndArtwork, so callers can handle WavPack identically to every
// other format.
func Read(f *os.File) (Tags, error) {
	items, _, err := readItems(f)
	if err != nil {
		return Tags{}, err
	}
	if items == nil {
		return Tags{}, nil
	}

	var t Tags
	for _, it := range items {
		switch strings.ToLower(it.key) {
		case "artist":
			t.Artist = string(it.value)
		case "album":
			t.Album = string(it.value)
		case "title":
			t.Title = string(it.value)
		case "genre":
			t.Genre = string(it.value)
		case "track":
			t.Track = parseLeadingInt(string(it.value))
		case "year":
			t.Year = parseLeadingInt(string(it.value))
		}
	}
	return t, nil
}

// parseLeadingInt reads the leading run of digits from s (handling
// Vorbis-style "N/M" track numbers, or a plain year), returning 0 if none.
func parseLeadingInt(s string) int {
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return n
}

// readItems returns every item in path's f's APEv2 tag (nil, not an error,
// if there isn't one) and the total on-disk byte length of the existing
// tag (items + footer, excluding any header — the length a caller needs to
// truncate off before appending a replacement).
func readItems(f *os.File) ([]item, int64, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	if info.Size() < footerSize {
		return nil, 0, nil
	}

	footer := make([]byte, footerSize)
	if _, err := f.ReadAt(footer, info.Size()-footerSize); err != nil {
		return nil, 0, fmt.Errorf("reading APEv2 footer: %w", err)
	}
	if string(footer[0:8]) != preamble {
		return nil, 0, nil
	}

	tagLength := binary.LittleEndian.Uint32(footer[12:16])
	count := binary.LittleEndian.Uint32(footer[16:20])
	if int64(tagLength) > info.Size() || tagLength < footerSize {
		return nil, 0, fmt.Errorf("invalid APEv2 tag length")
	}

	body := make([]byte, tagLength-footerSize)
	if _, err := f.ReadAt(body, info.Size()-int64(tagLength)); err != nil {
		return nil, 0, fmt.Errorf("reading APEv2 tag body: %w", err)
	}

	items := make([]item, 0, count)
	pos := 0
	for i := uint32(0); i < count; i++ {
		if pos+8 > len(body) {
			return nil, 0, fmt.Errorf("truncated APEv2 item")
		}
		valueSize := binary.LittleEndian.Uint32(body[pos : pos+4])
		flags := binary.LittleEndian.Uint32(body[pos+4 : pos+8])
		pos += 8

		nul := bytes.IndexByte(body[pos:], 0)
		if nul < 0 {
			return nil, 0, fmt.Errorf("unterminated APEv2 item key")
		}
		key := string(body[pos : pos+nul])
		pos += nul + 1

		if pos+int(valueSize) > len(body) {
			return nil, 0, fmt.Errorf("APEv2 item value overruns tag body")
		}
		value := body[pos : pos+int(valueSize)]
		pos += int(valueSize)

		items = append(items, item{key: key, value: value, valueType: int((flags >> 1) & 0x3)})
	}

	return items, int64(tagLength), nil
}

// Write rewrites path's APEv2 tag with t's five text fields, preserving
// every other existing item (Genre, Cover Art, or anything else) exactly
// as it was — mirroring how organize.setVorbisComment overwrites only the
// FLAC Vorbis-comment keys it's given. Creates a fresh tag if none existed.
func Write(path string, t Tags) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	existing, oldTagLength, err := readItems(f)
	if err != nil {
		return err
	}

	kept := existing[:0]
	skip := map[string]bool{"artist": true, "album": true, "title": true, "track": true, "year": true}
	for _, it := range existing {
		if !skip[strings.ToLower(it.key)] {
			kept = append(kept, it)
		}
	}

	kept = setTextItem(kept, "Artist", t.Artist)
	kept = setTextItem(kept, "Album", t.Album)
	kept = setTextItem(kept, "Title", t.Title)
	if t.Track > 0 {
		kept = setTextItem(kept, "Track", strconv.Itoa(t.Track))
	}
	if t.Year > 0 {
		kept = setTextItem(kept, "Year", strconv.Itoa(t.Year))
	}

	newTag := serializeTag(kept)

	info, err := f.Stat()
	if err != nil {
		return err
	}
	newSize := info.Size() - oldTagLength
	if err := f.Truncate(newSize); err != nil {
		return err
	}
	if _, err := f.WriteAt(newTag, newSize); err != nil {
		return err
	}
	return nil
}

func setTextItem(items []item, key, value string) []item {
	if value == "" {
		return items
	}
	return append(items, item{key: key, value: []byte(value), valueType: valueTypeUTF8})
}

func serializeTag(items []item) []byte {
	var body bytes.Buffer
	for _, it := range items {
		var sizeBuf, flagsBuf [4]byte
		binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(it.value)))
		binary.LittleEndian.PutUint32(flagsBuf[:], uint32(it.valueType&0x3)<<1)
		body.Write(sizeBuf[:])
		body.Write(flagsBuf[:])
		body.WriteString(it.key)
		body.WriteByte(0)
		body.Write(it.value)
	}

	footer := make([]byte, footerSize)
	copy(footer[0:8], preamble)
	binary.LittleEndian.PutUint32(footer[8:12], 2000)
	binary.LittleEndian.PutUint32(footer[12:16], uint32(body.Len()+footerSize))
	binary.LittleEndian.PutUint32(footer[16:20], uint32(len(items)))
	binary.LittleEndian.PutUint32(footer[20:24], 1<<30) // "has footer", no header

	return append(body.Bytes(), footer...)
}
