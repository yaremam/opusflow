// Package enrich fills in artwork and descriptive info for artists and
// albums that the local scan alone can't provide (artist photos, and album
// covers for untagged files) — embedded-tag art is extracted inline during
// scan.ExtractTags instead, since that's local and cheap; this package
// covers the network-dependent fallback and the facts/bio lookups that
// piggyback on the same MusicBrainz match. See TDR 003.
package enrich

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

// thumbSize/fullSize are the longer-side pixel bound for the grid-thumbnail
// and detail-hero image variants (TDR 003 §3, AC-15). An image already
// smaller than the bound is stored as-is, not upscaled.
const (
	thumbSize = 300
	fullSize  = 1000
)

// urlPrefix is the static route ImageStore's returned URLs are served
// under — see httpserver's artwork route, wired to the same ARTWORK_DIR.
const urlPrefix = "/artwork"

// ImageStore saves fetched/extracted cover art and photos to disk as two
// resized variants and returns URL paths for the caller to persist on the
// artist/album row — files on disk plus a DB path reference, not a bytea
// blob (TDR 003 §2).
type ImageStore struct {
	dir string
}

// NewImageStore builds an ImageStore rooted at dir (ARTWORK_DIR). dir is
// created if it doesn't already exist.
func NewImageStore(dir string) *ImageStore {
	return &ImageStore{dir: dir}
}

// Save decodes data as an image, writes a thumb.jpg and full.jpg variant
// under <dir>/<kind>/<id>/, and returns their URL paths for the caller to
// store on the artist/album row. kind is "artist" or "album".
func (st *ImageStore) Save(kind string, id int64, data []byte) (thumbURL, fullURL string, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("decoding image: %w", err)
	}

	relDir := filepath.Join(kind, strconv.FormatInt(id, 10))
	if err := os.MkdirAll(filepath.Join(st.dir, relDir), 0o755); err != nil {
		return "", "", fmt.Errorf("creating artwork directory: %w", err)
	}

	if err := writeVariant(filepath.Join(st.dir, relDir, "thumb.jpg"), img, thumbSize); err != nil {
		return "", "", err
	}
	if err := writeVariant(filepath.Join(st.dir, relDir, "full.jpg"), img, fullSize); err != nil {
		return "", "", err
	}

	urlDir := path.Join(urlPrefix, kind, strconv.FormatInt(id, 10))
	return urlDir + "/thumb.jpg", urlDir + "/full.jpg", nil
}
