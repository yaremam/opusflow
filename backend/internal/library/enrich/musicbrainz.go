package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

// musicBrainzRateLimit is the minimum gap between requests, per
// MusicBrainz's usage policy (https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting):
// "on average 1 request per second". A ticker-based limiter rather than a
// hard sleep so a burst that arrives more slowly than the limit never pays
// an unnecessary wait.
const musicBrainzRateLimit = time.Second

// musicBrainzBaseURL is the production API root; tests override it via
// MusicBrainz.baseURL to point at an httptest.Server instead.
const musicBrainzBaseURL = "https://musicbrainz.org/ws/2"

// MusicBrainz is a rate-limited client for the subset of the MusicBrainz
// API this package needs: searching artists/release-groups by name and
// looking up the facts (genres, country, formed year, label) and any
// linked Wikidata entity a matched entry carries. See TDR 003 — chosen
// over Spotify/Last.fm for being free, open, and requiring no API key,
// just a descriptive User-Agent per their usage policy.
type MusicBrainz struct {
	httpClient *http.Client
	limiter    *rateLimiter
	baseURL    string
	userAgent  string
}

// NewMusicBrainz builds a MusicBrainz client. userAgent should identify the
// application and a contact per MusicBrainz's usage policy (e.g.
// "opusflow/0.1 (https://github.com/yaremam/opusflow)") — requests without
// one are liable to be blocked.
func NewMusicBrainz(userAgent string) *MusicBrainz {
	return &MusicBrainz{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		limiter:    newRateLimiter(musicBrainzRateLimit),
		baseURL:    musicBrainzBaseURL,
		userAgent:  userAgent,
	}
}

// ArtistInfo is the facts and Wikidata link extracted from an artist
// lookup.
type ArtistInfo struct {
	FormedYear  int
	Country     string
	Genres      []string
	WikidataURL string // "" if the artist carries no wikidata relation
}

// ReleaseGroupInfo is the facts and Wikidata link extracted from a
// release-group lookup — opusflow's "album" maps to MusicBrainz's
// release-group, since a specific Release (pressing/edition) is finer
// grained than anything the local library models.
type ReleaseGroupInfo struct {
	Label       string
	Country     string
	Genres      []string
	WikidataURL string
}

// SearchArtist returns the top-ranked artist MBID matching name, or found
// = false if MusicBrainz has no match. No confidence threshold beyond
// MusicBrainz's own relevance ranking — see TDR 003's match-confidence
// decision.
func (m *MusicBrainz) SearchArtist(ctx context.Context, name string) (mbid string, found bool, err error) {
	var resp struct {
		Artists []struct {
			ID string `json:"id"`
		} `json:"artists"`
	}
	if err := m.get(ctx, "/artist", url.Values{"query": {name}, "fmt": {"json"}}, &resp); err != nil {
		return "", false, err
	}
	if len(resp.Artists) == 0 {
		return "", false, nil
	}
	return resp.Artists[0].ID, true, nil
}

// SearchReleaseGroup returns the top-ranked release-group MBID matching
// title by artist, or found = false if MusicBrainz has no match.
func (m *MusicBrainz) SearchReleaseGroup(ctx context.Context, title, artist string) (mbid string, found bool, err error) {
	var resp struct {
		ReleaseGroups []struct {
			ID string `json:"id"`
		} `json:"release-groups"`
	}
	query := fmt.Sprintf(`releasegroup:"%s" AND artist:"%s"`, title, artist)
	if err := m.get(ctx, "/release-group", url.Values{"query": {query}, "fmt": {"json"}}, &resp); err != nil {
		return "", false, err
	}
	if len(resp.ReleaseGroups) == 0 {
		return "", false, nil
	}
	return resp.ReleaseGroups[0].ID, true, nil
}

// LookupArtist fetches genres, country, formed year, and any Wikidata
// relation for the artist identified by mbid.
func (m *MusicBrainz) LookupArtist(ctx context.Context, mbid string) (ArtistInfo, error) {
	var resp struct {
		LifeSpan struct {
			Begin string `json:"begin"`
		} `json:"life-span"`
		Area struct {
			Name string `json:"name"`
		} `json:"area"`
		Genres    []mbGenre    `json:"genres"`
		Relations []mbRelation `json:"relations"`
	}
	params := url.Values{"inc": {"genres+url-rels"}, "fmt": {"json"}}
	if err := m.get(ctx, "/artist/"+mbid, params, &resp); err != nil {
		return ArtistInfo{}, err
	}
	return ArtistInfo{
		FormedYear:  parseYearPrefix(resp.LifeSpan.Begin),
		Country:     resp.Area.Name,
		Genres:      genreNames(resp.Genres),
		WikidataURL: wikidataURL(resp.Relations),
	}, nil
}

// LookupReleaseGroup fetches genres, any Wikidata relation, and (from its
// earliest associated release) country and label for the release-group
// identified by mbid.
func (m *MusicBrainz) LookupReleaseGroup(ctx context.Context, mbid string) (ReleaseGroupInfo, error) {
	var resp struct {
		Genres    []mbGenre    `json:"genres"`
		Relations []mbRelation `json:"relations"`
		Releases  []struct {
			Country   string `json:"country"`
			LabelInfo []struct {
				Label struct {
					Name string `json:"name"`
				} `json:"label"`
			} `json:"label-info"`
		} `json:"releases"`
	}
	params := url.Values{"inc": {"genres+url-rels+releases+labels"}, "fmt": {"json"}}
	if err := m.get(ctx, "/release-group/"+mbid, params, &resp); err != nil {
		return ReleaseGroupInfo{}, err
	}

	info := ReleaseGroupInfo{
		Genres:      genreNames(resp.Genres),
		WikidataURL: wikidataURL(resp.Relations),
	}
	if len(resp.Releases) > 0 {
		info.Country = resp.Releases[0].Country
		if len(resp.Releases[0].LabelInfo) > 0 {
			info.Label = resp.Releases[0].LabelInfo[0].Label.Name
		}
	}
	return info, nil
}

type mbGenre struct {
	Name string `json:"name"`
}

type mbRelation struct {
	Type string `json:"type"`
	URL  struct {
		Resource string `json:"resource"`
	} `json:"url"`
}

func genreNames(genres []mbGenre) []string {
	names := make([]string, 0, len(genres))
	for _, g := range genres {
		names = append(names, g.Name)
	}
	return names
}

func wikidataURL(relations []mbRelation) string {
	for _, r := range relations {
		if r.Type == "wikidata" {
			return r.URL.Resource
		}
	}
	return ""
}

// yearPrefix matches the leading 4-digit year in a MusicBrainz partial
// ISO8601 date ("2011", "2011-05", or "2011-05-01" are all valid).
var yearPrefix = regexp.MustCompile(`^\d{4}`)

func parseYearPrefix(date string) int {
	m := yearPrefix.FindString(date)
	if m == "" {
		return 0
	}
	var year int
	fmt.Sscanf(m, "%d", &year)
	return year
}

// get rate-limits, sends, and JSON-decodes a MusicBrainz GET request. A 404
// (not found) decodes as a zero-value resp rather than an error — callers
// that need to distinguish "not found" from "found with empty fields" do so
// via the caller-level Job logic (a search returning no results), not here.
func (m *MusicBrainz) get(ctx context.Context, path string, params url.Values, out any) error {
	if err := m.limiter.Wait(ctx); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("building musicbrainz request: %w", err)
	}
	req.Header.Set("User-Agent", m.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("musicbrainz request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("musicbrainz request: unexpected status %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding musicbrainz response: %w", err)
	}
	return nil
}
