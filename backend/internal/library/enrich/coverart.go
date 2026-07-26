package enrich

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

// FetchFront downloads the front cover image for releaseGroupMBID.
// Cover Art Archive's release-group endpoint redirects to whichever
// release actually has art — http.Client follows that automatically, so a
// 200 here always means real image bytes. found = false (nil error) means
// Cover Art Archive has nothing for this release-group at all.
func (c *CoverArtArchive) FetchFront(ctx context.Context, releaseGroupMBID string) (data []byte, found bool, err error) {
	resp, err := doGet(ctx, c.httpClient, c.userAgent, "", c.baseURL+"/release-group/"+releaseGroupMBID+"/front")
	if err != nil {
		return nil, false, fmt.Errorf("cover art archive request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("cover art archive request: unexpected status %s", resp.Status)
	}
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("reading cover art archive response: %w", err)
	}
	return data, true, nil
}
