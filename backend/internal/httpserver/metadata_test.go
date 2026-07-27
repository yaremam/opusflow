package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// fakeMetadataSearch is a library.MetadataSearch fake for testing the
// metadata-lookup endpoints without a real MusicBrainz client or database —
// these handlers never touch the store, only svc's MetadataSearch.
type fakeMetadataSearch struct {
	gotName             string
	gotArtistMBID       string
	gotReleaseGroupMBID string
	gotReleaseMBID      string
}

func (f *fakeMetadataSearch) SearchArtists(ctx context.Context, name string) ([]enrich.ArtistMatch, error) {
	f.gotName = name
	return []enrich.ArtistMatch{{MBID: "a1", Name: "Океан Ельзи", Disambiguation: "Ukrainian rock band"}}, nil
}

func (f *fakeMetadataSearch) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]enrich.ReleaseGroupMatch, error) {
	f.gotArtistMBID = artistMBID
	return []enrich.ReleaseGroupMatch{{MBID: "g1", Title: "Гегемонія", FirstReleaseYear: 2013}}, nil
}

func (f *fakeMetadataSearch) ReleaseGroupReleases(ctx context.Context, releaseGroupMBID string) ([]enrich.ReleaseMatch, error) {
	f.gotReleaseGroupMBID = releaseGroupMBID
	return []enrich.ReleaseMatch{{MBID: "r1", Country: "UA", Date: "2013-11-15", TrackCount: 12}}, nil
}

func (f *fakeMetadataSearch) ReleaseTracks(ctx context.Context, releaseMBID string) ([]enrich.Track, error) {
	f.gotReleaseMBID = releaseMBID
	return []enrich.Track{{Position: 1, Title: "Друге дихання"}}, nil
}

func newTestServiceWithMetadataSearch(fake *fakeMetadataSearch) *library.Service {
	svc := library.NewService(nil, nil)
	svc.SetMusicBrainzSearch(fake)
	return svc
}

func TestSearchArtistsEndpointReturnsMatches(t *testing.T) {
	fake := &fakeMetadataSearch{}
	svc := newTestServiceWithMetadataSearch(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/artists?q=%D0%9E%D0%BA%D0%B5%D0%B0%D0%BD", nil)
	w := httptest.NewRecorder()
	handleSearchArtists(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if fake.gotName != "Океан" {
		t.Fatalf("gotName = %q", fake.gotName)
	}
	if !strings.Contains(w.Body.String(), "Океан Ельзи") {
		t.Fatalf("body = %s, want it to contain the matched artist name", w.Body.String())
	}
}

func TestSearchArtistsEndpointRequiresQuery(t *testing.T) {
	svc := newTestServiceWithMetadataSearch(&fakeMetadataSearch{})

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/artists", nil)
	w := httptest.NewRecorder()
	handleSearchArtists(svc)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSearchArtistsEndpointWithoutConfigurationReturns503(t *testing.T) {
	svc := library.NewService(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/artists?q=test", nil)
	w := httptest.NewRecorder()
	handleSearchArtists(svc)(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestArtistReleaseGroupsEndpointReturnsGroups(t *testing.T) {
	fake := &fakeMetadataSearch{}
	svc := newTestServiceWithMetadataSearch(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/artists/artist-mbid-1/release-groups", nil)
	req.SetPathValue("mbid", "artist-mbid-1")
	w := httptest.NewRecorder()
	handleArtistReleaseGroups(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if fake.gotArtistMBID != "artist-mbid-1" {
		t.Fatalf("gotArtistMBID = %q", fake.gotArtistMBID)
	}
	if !strings.Contains(w.Body.String(), "Гегемонія") {
		t.Fatalf("body = %s", w.Body.String())
	}
}

func TestReleaseGroupReleasesEndpointReturnsReleases(t *testing.T) {
	fake := &fakeMetadataSearch{}
	svc := newTestServiceWithMetadataSearch(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/release-groups/rg-mbid-1/releases", nil)
	req.SetPathValue("mbid", "rg-mbid-1")
	w := httptest.NewRecorder()
	handleReleaseGroupReleases(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if fake.gotReleaseGroupMBID != "rg-mbid-1" {
		t.Fatalf("gotReleaseGroupMBID = %q", fake.gotReleaseGroupMBID)
	}
	if !strings.Contains(w.Body.String(), `"trackCount":12`) {
		t.Fatalf("body = %s, want trackCount 12", w.Body.String())
	}
}

func TestReleaseTracksEndpointReturnsTracks(t *testing.T) {
	fake := &fakeMetadataSearch{}
	svc := newTestServiceWithMetadataSearch(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/metadata/releases/release-mbid-1/tracks", nil)
	req.SetPathValue("mbid", "release-mbid-1")
	w := httptest.NewRecorder()
	handleReleaseTracks(svc)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if fake.gotReleaseMBID != "release-mbid-1" {
		t.Fatalf("gotReleaseMBID = %q", fake.gotReleaseMBID)
	}
	if !strings.Contains(w.Body.String(), "Друге дихання") {
		t.Fatalf("body = %s", w.Body.String())
	}
}
