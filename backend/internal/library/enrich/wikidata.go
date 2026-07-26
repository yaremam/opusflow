package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const (
	wikidataAPIBaseURL   = "https://www.wikidata.org/w/api.php"
	commonsFilePathBase  = "https://commons.wikimedia.org/wiki/Special:FilePath"
	wikipediaSummaryBase = "https://en.wikipedia.org/api/rest_v1/page/summary"
)

// qidPattern extracts the entity ID from a MusicBrainz "wikidata" relation
// URL, e.g. "https://www.wikidata.org/wiki/Q12345" -> "Q12345".
var qidPattern = regexp.MustCompile(`Q\d+$`)

// Wikidata resolves a MusicBrainz artist/release-group's linked Wikidata
// entity into a Commons photo filename (P18) and an English Wikipedia
// sitelink, then fetches each. Two hosts, two different APIs, chained
// together for one purpose: this is the second hop artist photos and
// bios/descriptions need beyond MusicBrainz itself (TDR 003 §2's "external
// service" alternative) — Wikidata carries no images or article text
// itself, only the links to Commons and Wikipedia that do.
type Wikidata struct {
	httpClient       *http.Client
	apiBaseURL       string
	commonsBaseURL   string
	wikipediaBaseURL string
	userAgent        string
}

// NewWikidata builds a Wikidata client.
func NewWikidata(userAgent string) *Wikidata {
	return &Wikidata{
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		apiBaseURL:       wikidataAPIBaseURL,
		commonsBaseURL:   commonsFilePathBase,
		wikipediaBaseURL: wikipediaSummaryBase,
		userAgent:        userAgent,
	}
}

// Entity is what ResolveEntity extracts from a Wikidata item: at most one
// Commons image filename (P18) and one English Wikipedia article title —
// either or both may be empty ("this entity has a Wikidata item but no
// enwiki article" is common).
type Entity struct {
	ImageFilename  string
	WikipediaTitle string
}

// ResolveEntity extracts the QID from wikidataURL (a MusicBrainz
// "wikidata" relation target) and fetches its P18 image claim and enwiki
// sitelink.
func (w *Wikidata) ResolveEntity(ctx context.Context, wikidataURL string) (Entity, error) {
	qid := qidPattern.FindString(wikidataURL)
	if qid == "" {
		return Entity{}, fmt.Errorf("no wikidata QID found in %q", wikidataURL)
	}

	params := url.Values{
		"action": {"wbgetentities"},
		"ids":    {qid},
		"props":  {"sitelinks|claims"},
		"format": {"json"},
	}
	var resp struct {
		Entities map[string]struct {
			Sitelinks map[string]struct {
				Title string `json:"title"`
			} `json:"sitelinks"`
			Claims struct {
				P18 []struct {
					Mainsnak struct {
						Datavalue struct {
							Value string `json:"value"`
						} `json:"datavalue"`
					} `json:"mainsnak"`
				} `json:"P18"`
			} `json:"claims"`
		} `json:"entities"`
	}
	if err := w.get(ctx, w.apiBaseURL, params, &resp); err != nil {
		return Entity{}, err
	}

	entity, ok := resp.Entities[qid]
	if !ok {
		return Entity{}, nil
	}

	var e Entity
	if len(entity.Claims.P18) > 0 {
		e.ImageFilename = entity.Claims.P18[0].Mainsnak.Datavalue.Value
	}
	if link, ok := entity.Sitelinks["enwiki"]; ok {
		e.WikipediaTitle = link.Title
	}
	return e, nil
}

// FetchImage downloads the Commons file named filename (as returned by
// ResolveEntity's ImageFilename) via Special:FilePath, which redirects to
// the actual file — http.Client follows that automatically.
func (w *Wikidata) FetchImage(ctx context.Context, filename string) ([]byte, error) {
	reqURL := w.commonsBaseURL + "/" + url.PathEscape(filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building commons request: %w", err)
	}
	req.Header.Set("User-Agent", w.userAgent)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("commons request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("commons request: unexpected status %s", resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading commons response: %w", err)
	}
	return data, nil
}

// Summary is a short prose extract of a Wikipedia article, plus the
// article's own URL for the "via Wikipedia" attribution AC-7 requires.
type Summary struct {
	Extract string
	URL     string
}

// FetchSummary fetches the lead-paragraph extract and canonical article URL
// for the given English Wikipedia title (as returned by ResolveEntity's
// WikipediaTitle), via Wikipedia's REST summary endpoint.
func (w *Wikidata) FetchSummary(ctx context.Context, title string) (Summary, error) {
	reqURL := w.wikipediaBaseURL + "/" + url.PathEscape(title)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return Summary{}, fmt.Errorf("building wikipedia summary request: %w", err)
	}
	req.Header.Set("User-Agent", w.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return Summary{}, fmt.Errorf("wikipedia summary request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Summary{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Summary{}, fmt.Errorf("wikipedia summary request: unexpected status %s", resp.Status)
	}

	var body struct {
		Extract     string `json:"extract"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Summary{}, fmt.Errorf("decoding wikipedia summary response: %w", err)
	}
	return Summary{Extract: body.Extract, URL: body.ContentURLs.Desktop.Page}, nil
}

func (w *Wikidata) get(ctx context.Context, base string, params url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("building wikidata request: %w", err)
	}
	req.Header.Set("User-Agent", w.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wikidata request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wikidata request: unexpected status %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding wikidata response: %w", err)
	}
	return nil
}
