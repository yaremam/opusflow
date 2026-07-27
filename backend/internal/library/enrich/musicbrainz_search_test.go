package enrich

import (
	"context"
	"net/http"
	"testing"
)

func TestSearchArtistsReturnsEveryMatch(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "Oceana" {
			t.Fatalf("query = %q, want Oceana", got)
		}
		w.Write([]byte(`{"artists": [
			{"id": "a1", "name": "Океан Ельзи", "disambiguation": "Ukrainian rock band"},
			{"id": "a2", "name": "Oceana", "disambiguation": "Belgian pop duo"},
			{"id": "a3", "name": "Ocean Alley"}
		]}`))
	})

	matches, err := m.SearchArtists(context.Background(), "Oceana")
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("len(matches) = %d, want 3", len(matches))
	}
	if matches[0].MBID != "a1" || matches[0].Name != "Океан Ельзи" || matches[0].Disambiguation != "Ukrainian rock band" {
		t.Fatalf("matches[0] = %+v", matches[0])
	}
	if matches[2].Disambiguation != "" {
		t.Fatalf("matches[2].Disambiguation = %q, want empty when absent", matches[2].Disambiguation)
	}
}

func TestSearchArtistsNoMatches(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": []}`))
	})

	matches, err := m.SearchArtists(context.Background(), "Nobody")
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("len(matches) = %d, want 0", len(matches))
	}
}

func TestArtistReleaseGroupsBrowsesByArtistMBID(t *testing.T) {
	var gotArtist, gotQuery string
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		gotArtist = r.URL.Query().Get("artist")
		gotQuery = r.URL.Query().Get("query")
		w.Write([]byte(`{"release-groups": [
			{"id": "g1", "title": "Гегемонія", "first-release-date": "2013-11-15"},
			{"id": "g2", "title": "Земля", "first-release-date": "2013"},
			{"id": "g3", "title": "Undated Release"}
		]}`))
	})

	groups, err := m.ArtistReleaseGroups(context.Background(), "artist-mbid-1")
	if err != nil {
		t.Fatalf("ArtistReleaseGroups: %v", err)
	}
	if gotArtist != "artist-mbid-1" {
		t.Fatalf("artist param = %q, want artist-mbid-1", gotArtist)
	}
	if gotQuery != "" {
		t.Fatalf("query param = %q, want empty — this is a browse, not a text search", gotQuery)
	}
	if len(groups) != 3 {
		t.Fatalf("len(groups) = %d, want 3", len(groups))
	}
	if groups[0].MBID != "g1" || groups[0].Title != "Гегемонія" || groups[0].FirstReleaseYear != 2013 {
		t.Fatalf("groups[0] = %+v", groups[0])
	}
	if groups[2].FirstReleaseYear != 0 {
		t.Fatalf("groups[2].FirstReleaseYear = %d, want 0 for a missing date", groups[2].FirstReleaseYear)
	}
}

func TestReleaseGroupReleasesSumsTrackCountAcrossMedia(t *testing.T) {
	var gotReleaseGroup string
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		gotReleaseGroup = r.URL.Query().Get("release-group")
		w.Write([]byte(`{"releases": [
			{"id": "r1", "date": "2013-11-15", "country": "UA", "media": [{"track-count": 12}]},
			{"id": "r2", "date": "2014-06-01", "country": "UA", "media": [{"track-count": 8}, {"track-count": 7}]},
			{"id": "r3", "date": "2013-11-15", "country": ""}
		]}`))
	})

	releases, err := m.ReleaseGroupReleases(context.Background(), "rg-mbid-1")
	if err != nil {
		t.Fatalf("ReleaseGroupReleases: %v", err)
	}
	if gotReleaseGroup != "rg-mbid-1" {
		t.Fatalf("release-group param = %q", gotReleaseGroup)
	}
	if len(releases) != 3 {
		t.Fatalf("len(releases) = %d, want 3", len(releases))
	}
	if releases[0].TrackCount != 12 {
		t.Fatalf("releases[0].TrackCount = %d, want 12", releases[0].TrackCount)
	}
	if releases[1].TrackCount != 15 {
		t.Fatalf("releases[1].TrackCount = %d, want 15 (sum across two media)", releases[1].TrackCount)
	}
	if releases[2].TrackCount != 0 {
		t.Fatalf("releases[2].TrackCount = %d, want 0 when no media reported", releases[2].TrackCount)
	}
	if releases[2].Country != "" {
		t.Fatalf("releases[2].Country = %q, want empty", releases[2].Country)
	}
}

func TestReleaseTracksFlattensMediaInOrder(t *testing.T) {
	// Real MusicBrainz responses have been observed sending "position" as a
	// bare JSON number (not a string) — this fixture intentionally matches
	// that, since ReleaseTracks must decode it regardless (it doesn't use
	// MusicBrainz's own numbering anyway; see musicbrainz_search.go).
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"media": [
			{"tracks": [
				{"position": 1, "title": "Друге дихання"},
				{"position": 2, "title": "Небо твоїх очей"}
			]},
			{"tracks": [
				{"position": 1, "title": "Disc 2, track 1"}
			]}
		]}`))
	})

	tracks, err := m.ReleaseTracks(context.Background(), "release-mbid-1")
	if err != nil {
		t.Fatalf("ReleaseTracks: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("len(tracks) = %d, want 3 (flattened across both media)", len(tracks))
	}
	if tracks[0].Position != 1 || tracks[0].Title != "Друге дихання" {
		t.Fatalf("tracks[0] = %+v", tracks[0])
	}
	if tracks[2].Title != "Disc 2, track 1" {
		t.Fatalf("tracks[2] = %+v, want the second medium's track appended after the first", tracks[2])
	}
}

func TestReleaseTracksWithNoMedia(t *testing.T) {
	m, _ := newTestMusicBrainz(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"media": []}`))
	})

	tracks, err := m.ReleaseTracks(context.Background(), "release-mbid-2")
	if err != nil {
		t.Fatalf("ReleaseTracks: %v", err)
	}
	if len(tracks) != 0 {
		t.Fatalf("len(tracks) = %d, want 0", len(tracks))
	}
}
