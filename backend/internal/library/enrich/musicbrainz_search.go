package enrich

import (
	"context"
	"net/url"
)

// ArtistMatch is one candidate from an interactive artist search (TDR
// 012) — unlike SearchArtist (which the background job uses and only
// keeps the top-ranked result), a person picking from a list needs every
// plausible match plus enough detail (Disambiguation) to tell same-named
// artists apart.
type ArtistMatch struct {
	MBID           string `json:"mbid"`
	Name           string `json:"name"`
	Disambiguation string `json:"disambiguation,omitempty"`
}

// ReleaseGroupMatch is one of an artist's albums (MusicBrainz
// release-group) — opusflow's "album" maps to a release-group, but a
// release-group carries no track listing itself (see ReleaseMatch).
type ReleaseGroupMatch struct {
	MBID             string `json:"mbid"`
	Title            string `json:"title"`
	FirstReleaseYear int    `json:"firstReleaseYear,omitempty"`
}

// ReleaseMatch is one specific pressing/edition of a release-group — the
// level track listings actually live at, since different editions
// (region, reissue, bonus tracks) can have different track counts.
type ReleaseMatch struct {
	MBID       string `json:"mbid"`
	Country    string `json:"country,omitempty"`
	Date       string `json:"date,omitempty"`
	TrackCount int    `json:"trackCount"`
}

// Track is one entry in a release's track listing.
type Track struct {
	Position int    `json:"position"`
	Title    string `json:"title"`
}

// SearchArtists returns every artist MusicBrainz's name search matches, in
// its own relevance-ranked order — the interactive counterpart to
// SearchArtist, which only keeps the first result for the background job.
func (m *MusicBrainz) SearchArtists(ctx context.Context, name string) ([]ArtistMatch, error) {
	var resp struct {
		Artists []struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			Disambiguation string `json:"disambiguation"`
		} `json:"artists"`
	}
	if err := m.get(ctx, "/artist", url.Values{"query": {name}, "fmt": {"json"}}, &resp); err != nil {
		return nil, err
	}
	matches := make([]ArtistMatch, 0, len(resp.Artists))
	for _, a := range resp.Artists {
		matches = append(matches, ArtistMatch{MBID: a.ID, Name: a.Name, Disambiguation: a.Disambiguation})
	}
	return matches, nil
}

// ArtistReleaseGroups browses every release-group MusicBrainz has on
// record for the artist identified by artistMBID — a browse query
// (?artist=) rather than a text search, so it can't return a
// wrong-artist result the way a name search could.
func (m *MusicBrainz) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]ReleaseGroupMatch, error) {
	var resp struct {
		ReleaseGroups []struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			FirstReleaseDate string `json:"first-release-date"`
		} `json:"release-groups"`
	}
	params := url.Values{"artist": {artistMBID}, "fmt": {"json"}}
	if err := m.get(ctx, "/release-group", params, &resp); err != nil {
		return nil, err
	}
	groups := make([]ReleaseGroupMatch, 0, len(resp.ReleaseGroups))
	for _, g := range resp.ReleaseGroups {
		groups = append(groups, ReleaseGroupMatch{
			MBID:             g.ID,
			Title:            g.Title,
			FirstReleaseYear: parseYearPrefix(g.FirstReleaseDate),
		})
	}
	return groups, nil
}

// ReleaseGroupReleases browses every specific release (pressing/edition)
// under the release-group identified by releaseGroupMBID, with each
// release's total track count (summed across its media/discs) so a
// picker can show which edition matches what the user actually has.
func (m *MusicBrainz) ReleaseGroupReleases(ctx context.Context, releaseGroupMBID string) ([]ReleaseMatch, error) {
	var resp struct {
		Releases []struct {
			ID      string `json:"id"`
			Date    string `json:"date"`
			Country string `json:"country"`
			Media   []struct {
				TrackCount int `json:"track-count"`
			} `json:"media"`
		} `json:"releases"`
	}
	params := url.Values{"release-group": {releaseGroupMBID}, "inc": {"media"}, "fmt": {"json"}}
	if err := m.get(ctx, "/release", params, &resp); err != nil {
		return nil, err
	}
	releases := make([]ReleaseMatch, 0, len(resp.Releases))
	for _, r := range resp.Releases {
		total := 0
		for _, med := range r.Media {
			total += med.TrackCount
		}
		releases = append(releases, ReleaseMatch{MBID: r.ID, Country: r.Country, Date: r.Date, TrackCount: total})
	}
	return releases, nil
}

// ReleaseTracks fetches the full track listing for the release identified
// by releaseMBID, flattening every medium (disc) into one ordered list —
// track matching (TDR 012) pairs these against a plan's files by position
// in this list, not by MusicBrainz's own per-medium track numbering.
func (m *MusicBrainz) ReleaseTracks(ctx context.Context, releaseMBID string) ([]Track, error) {
	var resp struct {
		Media []struct {
			Tracks []struct {
				// Position isn't decoded here — MusicBrainz has been
				// observed returning it as either a JSON number or a
				// string depending on the release, and this method
				// doesn't need MusicBrainz's own per-medium numbering
				// anyway (it assigns its own sequential Position below,
				// flattened across every medium).
				Title string `json:"title"`
			} `json:"tracks"`
		} `json:"media"`
	}
	params := url.Values{"inc": {"recordings"}, "fmt": {"json"}}
	if err := m.get(ctx, "/release/"+releaseMBID, params, &resp); err != nil {
		return nil, err
	}
	var tracks []Track
	position := 0
	for _, medium := range resp.Media {
		for _, tr := range medium.Tracks {
			position++
			tracks = append(tracks, Track{Position: position, Title: tr.Title})
		}
	}
	return tracks, nil
}
