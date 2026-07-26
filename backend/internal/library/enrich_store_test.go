package library

import (
	"errors"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

// insertTrackFor is a small helper: InsertTrack upserts the artist/album
// rows a test needs, exercising the same path a real import uses.
func insertTrackFor(t *testing.T, s *Store, artist, album string) {
	t.Helper()
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID: mustCreateImport(t, s),
		Path:     "/music/" + artist + "/" + album + "/track.mp3",
		Title:    "Track",
		Artist:   artist,
		Album:    album,
	}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
}

func TestArtistsNeedingEnrichmentExcludesUnknownArtist(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Real Artist", "Some Album")
	insertTrackFor(t, s, "", "") // Unknown Artist / Unknown Album

	targets, err := s.ArtistsNeedingEnrichment(ctx(), 100)
	if err != nil {
		t.Fatalf("ArtistsNeedingEnrichment: %v", err)
	}
	for _, target := range targets {
		if target.Name == "" {
			t.Fatalf("expected Unknown Artist to be excluded, got %+v", target)
		}
	}

	found := false
	for _, target := range targets {
		if target.Name == "Real Artist" {
			found = true
			if target.ArtStatus != enrich.Pending || target.FactsStatus != enrich.Pending || target.BioStatus != enrich.Pending {
				t.Fatalf("expected all-pending for a freshly-inserted artist, got %+v", target)
			}
		}
	}
	if !found {
		t.Fatal("expected Real Artist in the needing-enrichment list")
	}
}

func TestAlbumsNeedingEnrichmentExcludesUnknownAlbum(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Real Artist", "Real Album")
	insertTrackFor(t, s, "Real Artist", "")

	targets, err := s.AlbumsNeedingEnrichment(ctx(), 100)
	if err != nil {
		t.Fatalf("AlbumsNeedingEnrichment: %v", err)
	}
	for _, target := range targets {
		if target.Title == "" {
			t.Fatalf("expected Unknown Album to be excluded, got %+v", target)
		}
	}
}

func TestSetArtistFactsWithNilGenresDoesNotViolateNotNull(t *testing.T) {
	// Job passes nil genres for an artist whose MusicBrainz lookup itself
	// failed or never resolved (markArtistUnresolved's Failed/NotFound
	// paths) — genres is a NOT NULL TEXT[] column, so this must coerce to
	// an empty array rather than erroring.
	s := testStore(t)
	insertTrackFor(t, s, "Nil Genres Artist", "Album")
	artist := findArtistByName(t, s, "Nil Genres Artist")

	if err := s.SetArtistFacts(ctx(), artist.ID, enrich.Failed, enrich.ArtistInfo{}); err != nil {
		t.Fatalf("SetArtistFacts with nil genres: %v", err)
	}

	got, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got.Genres == nil || len(got.Genres) != 0 {
		t.Fatalf("Genres = %#v, want a non-nil empty slice", got.Genres)
	}
}

func TestSetAlbumFactsWithNilGenresDoesNotViolateNotNull(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Nil Genres Album")
	album := findAlbumByTitle(t, s, "Nil Genres Album")

	if err := s.SetAlbumFacts(ctx(), album.ID, enrich.Failed, enrich.ReleaseGroupInfo{}); err != nil {
		t.Fatalf("SetAlbumFacts with nil genres: %v", err)
	}

	got, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if got.Genres == nil || len(got.Genres) != 0 {
		t.Fatalf("Genres = %#v, want a non-nil empty slice", got.Genres)
	}
}

func TestSetArtistArtRoundTrips(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Photo Artist", "Album")
	artist := findArtistByName(t, s, "Photo Artist")

	if err := s.SetArtistMusicBrainzID(ctx(), artist.ID, "mbid-123"); err != nil {
		t.Fatalf("SetArtistMusicBrainzID: %v", err)
	}
	if err := s.SetArtistArt(ctx(), artist.ID, enrich.Found, "/artwork/artist/1/thumb.jpg", "/artwork/artist/1/full.jpg"); err != nil {
		t.Fatalf("SetArtistArt: %v", err)
	}

	got, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got.PhotoThumbURL != "/artwork/artist/1/thumb.jpg" || got.PhotoURL != "/artwork/artist/1/full.jpg" {
		t.Fatalf("got photo URLs %q / %q", got.PhotoThumbURL, got.PhotoURL)
	}

	targets, err := s.ArtistsNeedingEnrichment(ctx(), 100)
	if err != nil {
		t.Fatalf("ArtistsNeedingEnrichment: %v", err)
	}
	for _, target := range targets {
		if target.ID == artist.ID && target.MusicBrainzID != "mbid-123" {
			t.Fatalf("expected cached MBID, got %+v", target)
		}
		if target.ID == artist.ID && target.ArtStatus != enrich.Found {
			t.Fatalf("expected ArtStatus found after SetArtistArt, got %+v", target)
		}
	}
}

// TestSetArtistArtPreservesExistingPhotoOnNotFound guards TDR 007's AC-11:
// a retry that comes up empty this time must never destroy an
// already-found photo — only a Found write may touch the path columns.
func TestSetArtistArtPreservesExistingPhotoOnNotFound(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Retried Artist", "Album")
	artist := findArtistByName(t, s, "Retried Artist")

	if err := s.SetArtistArt(ctx(), artist.ID, enrich.Found, "/artwork/artist/1/thumb.jpg", "/artwork/artist/1/full.jpg"); err != nil {
		t.Fatalf("SetArtistArt(Found): %v", err)
	}
	if err := s.SetArtistArt(ctx(), artist.ID, enrich.NotFound, "", ""); err != nil {
		t.Fatalf("SetArtistArt(NotFound): %v", err)
	}

	got, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got.PhotoThumbURL != "/artwork/artist/1/thumb.jpg" || got.PhotoURL != "/artwork/artist/1/full.jpg" {
		t.Fatalf("photo URLs should survive a later NotFound write, got %q / %q", got.PhotoThumbURL, got.PhotoURL)
	}
}

func TestSetAlbumArtPreservesExistingCoverOnFailed(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Retried Album")
	album := findAlbumByTitle(t, s, "Retried Album")

	if err := s.SetAlbumArt(ctx(), album.ID, enrich.Found, "/artwork/album/1/thumb.jpg", "/artwork/album/1/full.jpg"); err != nil {
		t.Fatalf("SetAlbumArt(Found): %v", err)
	}
	if err := s.SetAlbumArt(ctx(), album.ID, enrich.Failed, "", ""); err != nil {
		t.Fatalf("SetAlbumArt(Failed): %v", err)
	}

	got, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if got.CoverThumbURL != "/artwork/album/1/thumb.jpg" || got.CoverURL != "/artwork/album/1/full.jpg" {
		t.Fatalf("cover URLs should survive a later Failed write, got %q / %q", got.CoverThumbURL, got.CoverURL)
	}
}

// TestResetArtistArtMarksPendingWithoutTouchingPhoto backs TDR 007's retry
// flow: resetting status to pending must leave any existing photo alone,
// so the last known-good image keeps rendering while a retry is in flight.
func TestResetArtistArtMarksPendingWithoutTouchingPhoto(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Reset Artist", "Album")
	artist := findArtistByName(t, s, "Reset Artist")

	if err := s.SetArtistArt(ctx(), artist.ID, enrich.Found, "/artwork/artist/1/thumb.jpg", "/artwork/artist/1/full.jpg"); err != nil {
		t.Fatalf("SetArtistArt: %v", err)
	}
	if err := s.ResetArtistArt(ctx(), artist.ID); err != nil {
		t.Fatalf("ResetArtistArt: %v", err)
	}

	got, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got.ArtStatus != enrich.Pending {
		t.Fatalf("ArtStatus = %q, want pending", got.ArtStatus)
	}
	if got.PhotoThumbURL != "/artwork/artist/1/thumb.jpg" || got.PhotoURL != "/artwork/artist/1/full.jpg" {
		t.Fatalf("photo URLs should survive a reset, got %q / %q", got.PhotoThumbURL, got.PhotoURL)
	}
}

func TestResetAlbumArtMarksPendingWithoutTouchingCover(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Reset Album")
	album := findAlbumByTitle(t, s, "Reset Album")

	if err := s.SetAlbumArt(ctx(), album.ID, enrich.Found, "/artwork/album/1/thumb.jpg", "/artwork/album/1/full.jpg"); err != nil {
		t.Fatalf("SetAlbumArt: %v", err)
	}
	if err := s.ResetAlbumArt(ctx(), album.ID); err != nil {
		t.Fatalf("ResetAlbumArt: %v", err)
	}

	got, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if got.ArtStatus != enrich.Pending {
		t.Fatalf("ArtStatus = %q, want pending", got.ArtStatus)
	}
	if got.CoverThumbURL != "/artwork/album/1/thumb.jpg" || got.CoverURL != "/artwork/album/1/full.jpg" {
		t.Fatalf("cover URLs should survive a reset, got %q / %q", got.CoverThumbURL, got.CoverURL)
	}
}

func TestResetArtistArtNotFound(t *testing.T) {
	s := testStore(t)
	if err := s.ResetArtistArt(ctx(), 999999); !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("ResetArtistArt(nonexistent) = %v, want ErrArtistNotFound", err)
	}
}

func TestResetAlbumArtNotFound(t *testing.T) {
	s := testStore(t)
	if err := s.ResetAlbumArt(ctx(), 999999); !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("ResetAlbumArt(nonexistent) = %v, want ErrAlbumNotFound", err)
	}
}

func TestSetArtistFactsAndBioRoundTrip(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Facts Artist", "Album")
	artist := findArtistByName(t, s, "Facts Artist")

	if err := s.SetArtistFacts(ctx(), artist.ID, enrich.Found, enrich.ArtistInfo{FormedYear: 1999, Country: "UK", Genres: []string{"Dream pop", "Shoegaze"}}); err != nil {
		t.Fatalf("SetArtistFacts: %v", err)
	}
	if err := s.SetArtistBio(ctx(), artist.ID, enrich.NotFound, "", ""); err != nil {
		t.Fatalf("SetArtistBio: %v", err)
	}

	got, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if got.FormedYear != 1999 || got.Country != "UK" {
		t.Fatalf("got facts %+v", got.Artist)
	}
	if len(got.Genres) != 2 || got.Genres[0] != "Dream pop" || got.Genres[1] != "Shoegaze" {
		t.Fatalf("got genres %+v", got.Genres)
	}
	if got.Bio != "" {
		t.Fatalf("expected empty bio after not_found, got %q", got.Bio)
	}

	// Facts (found) and bio (not_found) are both settled, but art was never
	// touched by this test — it must still show pending, and the artist
	// must still surface from the needing-enrichment query on that basis
	// alone. Each of the three kinds tracks its own status independently
	// (AC-8) rather than art/facts/bio all moving together.
	targets, err := s.ArtistsNeedingEnrichment(ctx(), 100)
	if err != nil {
		t.Fatalf("ArtistsNeedingEnrichment: %v", err)
	}
	var target *enrich.ArtistTarget
	for i := range targets {
		if targets[i].ID == artist.ID {
			target = &targets[i]
		}
	}
	if target == nil {
		t.Fatal("expected Facts Artist to still surface (art status still pending)")
	}
	if target.FactsStatus != enrich.Found || target.BioStatus != enrich.NotFound || target.ArtStatus != enrich.Pending {
		t.Fatalf("got %+v", target)
	}
}

func TestSetAlbumArtFactsDescriptionRoundTrip(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Album Artist", "Cover Album")
	album := findAlbumByTitle(t, s, "Cover Album")

	if err := s.SetAlbumMusicBrainzID(ctx(), album.ID, "rg-456"); err != nil {
		t.Fatalf("SetAlbumMusicBrainzID: %v", err)
	}
	if err := s.SetAlbumArt(ctx(), album.ID, enrich.Found, "/artwork/album/1/thumb.jpg", "/artwork/album/1/full.jpg"); err != nil {
		t.Fatalf("SetAlbumArt: %v", err)
	}
	if err := s.SetAlbumFacts(ctx(), album.ID, enrich.Found, enrich.ReleaseGroupInfo{Label: "Harbor & Kite", Country: "UK", Genres: []string{"Dream pop"}}); err != nil {
		t.Fatalf("SetAlbumFacts: %v", err)
	}
	if err := s.SetAlbumDescription(ctx(), album.ID, enrich.Found, "A record about tides.", "https://en.wikipedia.org/wiki/Example"); err != nil {
		t.Fatalf("SetAlbumDescription: %v", err)
	}

	got, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if got.CoverThumbURL != "/artwork/album/1/thumb.jpg" || got.CoverURL != "/artwork/album/1/full.jpg" {
		t.Fatalf("got cover URLs %q / %q", got.CoverThumbURL, got.CoverURL)
	}
	if got.Label != "Harbor & Kite" || got.Country != "UK" || len(got.Genres) != 1 {
		t.Fatalf("got facts %+v", got.Album)
	}
	if got.Description != "A record about tides." || got.DescriptionSourceURL == "" {
		t.Fatalf("got description %+v", got.Album)
	}
}

func findArtistByName(t *testing.T, s *Store, name string) Artist {
	t.Helper()
	page, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 100, Query: name})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	for _, a := range page.Items {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("artist %q not found", name)
	return Artist{}
}

func findAlbumByTitle(t *testing.T, s *Store, title string) Album {
	t.Helper()
	page, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 100, Query: title})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	for _, al := range page.Items {
		if al.Title == title {
			return al
		}
	}
	t.Fatalf("album %q not found", title)
	return Album{}
}
