package library

import (
	"context"
	"errors"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
)

// fakeMetadataSearch is a MetadataSearch fake recording what it was asked
// and returning canned results, for testing Service's passthroughs without
// a real MusicBrainz client.
type fakeMetadataSearch struct {
	gotName             string
	gotArtistMBID       string
	gotReleaseGroupMBID string
	gotReleaseMBID      string
	err                 error
}

func (f *fakeMetadataSearch) SearchArtists(ctx context.Context, name string) ([]enrich.ArtistMatch, error) {
	f.gotName = name
	if f.err != nil {
		return nil, f.err
	}
	return []enrich.ArtistMatch{{MBID: "a1", Name: "Океан Ельзи"}}, nil
}

func (f *fakeMetadataSearch) ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]enrich.ReleaseGroupMatch, error) {
	f.gotArtistMBID = artistMBID
	if f.err != nil {
		return nil, f.err
	}
	return []enrich.ReleaseGroupMatch{{MBID: "g1", Title: "Гегемонія", FirstReleaseYear: 2013}}, nil
}

func (f *fakeMetadataSearch) ReleaseGroupReleases(ctx context.Context, releaseGroupMBID string) ([]enrich.ReleaseMatch, error) {
	f.gotReleaseGroupMBID = releaseGroupMBID
	if f.err != nil {
		return nil, f.err
	}
	return []enrich.ReleaseMatch{{MBID: "r1", Country: "UA", TrackCount: 12}}, nil
}

func (f *fakeMetadataSearch) ReleaseTracks(ctx context.Context, releaseMBID string) ([]enrich.Track, error) {
	f.gotReleaseMBID = releaseMBID
	if f.err != nil {
		return nil, f.err
	}
	return []enrich.Track{{Position: 1, Title: "Друге дихання"}}, nil
}

func TestSearchArtistsDelegatesToMusicBrainzSearch(t *testing.T) {
	svc := NewService(newFakeImportStore(), newRecordingCopier())
	fake := &fakeMetadataSearch{}
	svc.SetMusicBrainzSearch(fake)

	matches, err := svc.SearchArtists(ctx(), "Океан Ельзи")
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if fake.gotName != "Океан Ельзи" {
		t.Fatalf("gotName = %q", fake.gotName)
	}
	if len(matches) != 1 || matches[0].MBID != "a1" {
		t.Fatalf("matches = %+v", matches)
	}
}

func TestArtistReleaseGroupsDelegatesToMusicBrainzSearch(t *testing.T) {
	svc := NewService(newFakeImportStore(), newRecordingCopier())
	fake := &fakeMetadataSearch{}
	svc.SetMusicBrainzSearch(fake)

	groups, err := svc.ArtistReleaseGroups(ctx(), "artist-mbid-1")
	if err != nil {
		t.Fatalf("ArtistReleaseGroups: %v", err)
	}
	if fake.gotArtistMBID != "artist-mbid-1" {
		t.Fatalf("gotArtistMBID = %q", fake.gotArtistMBID)
	}
	if len(groups) != 1 || groups[0].Title != "Гегемонія" {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestReleaseGroupReleasesDelegatesToMusicBrainzSearch(t *testing.T) {
	svc := NewService(newFakeImportStore(), newRecordingCopier())
	fake := &fakeMetadataSearch{}
	svc.SetMusicBrainzSearch(fake)

	releases, err := svc.ReleaseGroupReleases(ctx(), "rg-mbid-1")
	if err != nil {
		t.Fatalf("ReleaseGroupReleases: %v", err)
	}
	if fake.gotReleaseGroupMBID != "rg-mbid-1" {
		t.Fatalf("gotReleaseGroupMBID = %q", fake.gotReleaseGroupMBID)
	}
	if len(releases) != 1 || releases[0].TrackCount != 12 {
		t.Fatalf("releases = %+v", releases)
	}
}

func TestReleaseTracksDelegatesToMusicBrainzSearch(t *testing.T) {
	svc := NewService(newFakeImportStore(), newRecordingCopier())
	fake := &fakeMetadataSearch{}
	svc.SetMusicBrainzSearch(fake)

	tracks, err := svc.ReleaseTracks(ctx(), "release-mbid-1")
	if err != nil {
		t.Fatalf("ReleaseTracks: %v", err)
	}
	if fake.gotReleaseMBID != "release-mbid-1" {
		t.Fatalf("gotReleaseMBID = %q", fake.gotReleaseMBID)
	}
	if len(tracks) != 1 || tracks[0].Title != "Друге дихання" {
		t.Fatalf("tracks = %+v", tracks)
	}
}

func TestMetadataSearchMethodsWithoutConfigurationReturnError(t *testing.T) {
	svc := NewService(newFakeImportStore(), newRecordingCopier())

	if _, err := svc.SearchArtists(ctx(), "anything"); !errors.Is(err, ErrMetadataSearchNotConfigured) {
		t.Fatalf("SearchArtists err = %v, want ErrMetadataSearchNotConfigured", err)
	}
	if _, err := svc.ArtistReleaseGroups(ctx(), "mbid"); !errors.Is(err, ErrMetadataSearchNotConfigured) {
		t.Fatalf("ArtistReleaseGroups err = %v, want ErrMetadataSearchNotConfigured", err)
	}
	if _, err := svc.ReleaseGroupReleases(ctx(), "mbid"); !errors.Is(err, ErrMetadataSearchNotConfigured) {
		t.Fatalf("ReleaseGroupReleases err = %v, want ErrMetadataSearchNotConfigured", err)
	}
	if _, err := svc.ReleaseTracks(ctx(), "mbid"); !errors.Is(err, ErrMetadataSearchNotConfigured) {
		t.Fatalf("ReleaseTracks err = %v, want ErrMetadataSearchNotConfigured", err)
	}
}
