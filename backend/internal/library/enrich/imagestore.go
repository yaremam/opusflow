// Package enrich fills in artwork and descriptive info for artists and
// albums that the local scan alone can't provide (artist photos, and album
// covers for untagged files) — embedded-tag art is extracted inline during
// scan.ExtractTags instead, since that's local and cheap; this package
// covers the network-dependent fallback and the facts/bio lookups that
// piggyback on the same MusicBrainz match. See TDR 003.
package enrich

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
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
// under <dir>/<kind>/<id>/<contentHash>/, and returns their URL paths plus
// the hex SHA-256 of data (TDR 014) for the caller to pass to
// AddArtistPhoto/AddAlbumCover, which use it to skip re-adding a gallery
// entry for image bytes already present. Keying the on-disk path by hash
// too means the same image saved twice (e.g. two tracks on an album
// sharing one embedded cover) reuses one set of files rather than
// duplicating them. kind is "artist" or "album".
func (st *ImageStore) Save(kind string, id int64, data []byte) (thumbURL, fullURL, contentHash string, err error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", "", "", fmt.Errorf("decoding image: %w", err)
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	relDir := filepath.Join(kind, strconv.FormatInt(id, 10), hash)
	if err := os.MkdirAll(filepath.Join(st.dir, relDir), 0o755); err != nil {
		return "", "", "", fmt.Errorf("creating artwork directory: %w", err)
	}

	if err := writeVariant(filepath.Join(st.dir, relDir, "thumb.jpg"), img, thumbSize); err != nil {
		return "", "", "", err
	}
	if err := writeVariant(filepath.Join(st.dir, relDir, "full.jpg"), img, fullSize); err != nil {
		return "", "", "", err
	}

	urlDir := path.Join(urlPrefix, kind, strconv.FormatInt(id, 10), hash)
	return urlDir + "/thumb.jpg", urlDir + "/full.jpg", hash, nil
}

// Delete removes the on-disk thumb/full files a prior Save produced, plus
// their now-empty containing hash directory — each hash directory holds
// exactly these two files (see Save), so once both are gone the
// directory is safe to remove unconditionally rather than checking
// emptiness against unrelated content. Used when a gallery entry is
// individually removed (AC-4) and the reviewer chose to also delete the
// file, not just the catalog reference. A blank URL (nothing was ever
// saved for this entry, e.g. a legacy row) is silently skipped.
func (st *ImageStore) Delete(thumbURL, fullURL string) error {
	var hashDir string
	for _, u := range []string{thumbURL, fullURL} {
		if u == "" {
			continue
		}
		rel := filepath.FromSlash(strings.TrimPrefix(u, urlPrefix))
		full := filepath.Join(st.dir, rel)
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", full, err)
		}
		hashDir = filepath.Dir(full)
	}
	if hashDir != "" {
		if err := os.Remove(hashDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", hashDir, err)
		}
	}
	return nil
}
