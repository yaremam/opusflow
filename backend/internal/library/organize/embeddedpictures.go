package organize

import (
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/go-flac/flacpicture/v2"
	goflac "github.com/go-flac/go-flac/v2"

	"github.com/yaremam/opusflow/backend/internal/library/apev2"
)

// EmbeddedPicture is one picture found embedded in a track's own tags
// (TDR 014 AC-7) — a track can carry more than one (front cover, back
// cover, a booklet page, ...), unlike every other tag field this project
// reads via github.com/dhowden/tag, which keeps only the last-parsed
// picture per file for every format it supports.
type EmbeddedPicture struct {
	Data        []byte
	PictureType string
}

// pictureTypeLabels maps the numeric picture-type byte both ID3v2's APIC
// frame and FLAC's PICTURE metadata block use (the same table) to a
// lowercase label. Index 3 ("front") and 4 ("back") deliberately match
// Cover Art Archive's own vocabulary (enrich.Image.PictureType) — the one
// pair of types both an embedded tag and Cover Art Archive can genuinely
// agree on; the rest have no CAA equivalent and just get a clear label of
// their own.
var pictureTypeLabels = []string{
	"other", "file_icon", "other_icon", "front", "back", "leaflet", "media",
	"lead_artist", "artist", "conductor", "band", "composer", "lyricist",
	"recording_location", "during_recording", "during_performance",
	"screen_capture", "bright_coloured_fish", "illustration",
	"band_artist_logotype", "publisher_studio_logotype",
}

// pictureTypeLabel returns n's label, or "" for anything outside the
// standard table (a malformed or vendor-extended value) — an unrecognized
// type is still a real picture worth keeping, just with no type of its
// own, the same as a plain manual upload.
func pictureTypeLabel(n int) string {
	if n < 0 || n >= len(pictureTypeLabels) {
		return ""
	}
	return pictureTypeLabels[n]
}

// extractEmbeddedPictures returns every picture embedded in path's own
// tags, dispatched by extension — each container needs its own traversal,
// since dhowden/tag (used for every other tag field) only ever keeps one
// picture regardless of format. A format with no pictures at all (or no
// tags whatsoever) returns an empty slice, not an error.
func extractEmbeddedPictures(path string) ([]EmbeddedPicture, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return extractMP3Pictures(path)
	case ".flac":
		return extractFLACPictures(path)
	case ".wv":
		return extractWavPackPictures(path)
	case ".ogg":
		return extractOGGPictures(path)
	case ".m4a":
		return extractM4APictures(path)
	default:
		return nil, nil
	}
}

// extractMP3Pictures collects every APIC frame — id3v2.Tag.GetFrames
// returns all of them, unlike dhowden/tag's Picture() (last one wins).
func extractMP3Pictures(path string) ([]EmbeddedPicture, error) {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		return nil, nil
	}
	defer tag.Close()

	var pics []EmbeddedPicture
	for _, f := range tag.GetFrames("APIC") {
		pf, ok := f.(id3v2.PictureFrame)
		if !ok {
			continue
		}
		pics = append(pics, EmbeddedPicture{Data: pf.Picture, PictureType: pictureTypeLabel(int(pf.PictureType))})
	}
	return pics, nil
}

// extractFLACPictures collects every PICTURE metadata block — go-flac's
// f.Meta exposes every block (unlike dhowden/tag's single *Picture field),
// and flacpicture.ParseFromMetaDataBlock decodes each one using the same
// numeric picture-type table ID3v2 uses.
func extractFLACPictures(path string) ([]EmbeddedPicture, error) {
	f, err := goflac.ParseFile(path)
	if err != nil {
		return nil, nil
	}

	var pics []EmbeddedPicture
	for _, m := range f.Meta {
		if m.Type != goflac.Picture {
			continue
		}
		pic, err := flacpicture.ParseFromMetaDataBlock(*m)
		if err != nil {
			continue
		}
		pics = append(pics, EmbeddedPicture{Data: pic.ImageData, PictureType: pictureTypeLabel(int(pic.PictureType))})
	}
	return pics, nil
}

// extractWavPackPictures collects every "Cover Art (*)"-keyed APEv2 item —
// apev2.ReadArtworks (unlike apev2.Read's single Artwork field) returns
// all of them, each already carrying its picture type parsed from the
// item's own key ("Cover Art (Front)" -> "front", ...).
func extractWavPackPictures(path string) ([]EmbeddedPicture, error) {
	artworks, err := apev2.ReadArtworks(path)
	if err != nil {
		return nil, nil
	}
	pics := make([]EmbeddedPicture, len(artworks))
	for i, a := range artworks {
		pics[i] = EmbeddedPicture{Data: a.Data, PictureType: a.PictureType}
	}
	return pics, nil
}
