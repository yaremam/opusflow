package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

const coverArtArchiveBaseURL = "https://coverartarchive.org"

// CoverArtArchive fetches album cover images by MusicBrainz release-group
// ID. It's a separate host from the MusicBrainz API proper (backed by the
// Internet Archive) with its own, more lenient usage policy — no rate
// limiter here, just the same descriptive User-Agent convention.
type CoverArtArchive struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// NewCoverArtArchive builds a CoverArtArchive client.
func NewCoverArtArchive(userAgent string) *CoverArtArchive {
	return &CoverArtArchive{
		httpClient: newHTTPClient(),
		baseURL:    coverArtArchiveBaseURL,
		userAgent:  userAgent,
	}
}

// Image is one image Cover Art Archive has on record for a release,
// resolved to its actual bytes. PictureType is Cover Art Archive's own
// type label (front/back/booklet/medium/tray/obi/spine/track/liner/
// sticker/poster/watermark/raw/other), lowercased to match this app's
// picture_type convention — blank if Cover Art Archive reported no type
// for that image.
type Image struct {
	Data        []byte
	PictureType string
}

// caaReleaseGroupResponse mirrors the fields this package reads from Cover
// Art Archive's release-group JSON endpoint — everything else in the real
// response (approved, comment, edit, thumbnails, ...) is left unread since
// this app generates its own thumb/full variants from the full-size image.
type caaReleaseGroupResponse struct {
	Images []struct {
		Types []string `json:"types"`
		Image string   `json:"image"`
	} `json:"images"`
}

// FetchAll downloads every image Cover Art Archive has for
// releaseGroupMBID's matched release, each tagged with its own picture
// type (TDR 014 AC-6) — unlike the old front-only lookup, this hits the
// real /release-group/{mbid} endpoint directly rather than the /front
// convenience redirect. A release-group with no art at all (404, or a
// 200 with an empty images array) returns an empty slice, not an error.
// A single image that fails to download is logged and skipped rather
// than failing the whole fetch — every other image found is still worth
// having.
func (c *CoverArtArchive) FetchAll(ctx context.Context, releaseGroupMBID string) ([]Image, error) {
	resp, err := doGet(ctx, c.httpClient, c.userAgent, "application/json", c.baseURL+"/release-group/"+releaseGroupMBID)
	if err != nil {
		return nil, fmt.Errorf("cover art archive request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover art archive request: unexpected status %s", resp.Status)
	}

	var parsed caaReleaseGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding cover art archive response: %w", err)
	}

	images := make([]Image, 0, len(parsed.Images))
	for _, img := range parsed.Images {
		data, err := c.fetchImageBytes(ctx, img.Image)
		if err != nil {
			log.Printf("enrich: cover art archive: fetching %s: %v", img.Image, err)
			continue
		}
		pictureType := ""
		if len(img.Types) > 0 {
			pictureType = strings.ToLower(img.Types[0])
		}
		images = append(images, Image{Data: data, PictureType: pictureType})
	}
	return images, nil
}

// fetchImageBytes downloads one image's full-size bytes from its own
// absolute URL — Cover Art Archive's release-group response embeds these
// directly, potentially on a different host than the release-group
// endpoint itself, so this bypasses c.baseURL entirely.
func (c *CoverArtArchive) fetchImageBytes(ctx context.Context, imageURL string) ([]byte, error) {
	resp, err := doGet(ctx, c.httpClient, c.userAgent, "", imageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}
