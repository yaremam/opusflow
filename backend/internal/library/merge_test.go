package library

import (
	"errors"
	"testing"
)

func TestFindArtistIDByMusicBrainzIDFindsMatch(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist A", "Album")
	insertTrackFor(t, s, "Artist B", "Album")
	a := findArtistByName(t, s, "Artist A")
	b := findArtistByName(t, s, "Artist B")

	if err := s.SetArtistMusicBrainzID(ctx(), b.ID, "mbid-shared"); err != nil {
		t.Fatalf("SetArtistMusicBrainzID: %v", err)
	}

	id, ok, err := s.FindArtistIDByMusicBrainzID(ctx(), "mbid-shared", a.ID)
	if err != nil {
		t.Fatalf("FindArtistIDByMusicBrainzID: %v", err)
	}
	if !ok || id != b.ID {
		t.Fatalf("id, ok = %d, %v, want %d, true", id, ok, b.ID)
	}
}

func TestFindArtistIDByMusicBrainzIDExcludesGivenID(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist A", "Album")
	a := findArtistByName(t, s, "Artist A")
	if err := s.SetArtistMusicBrainzID(ctx(), a.ID, "mbid-solo"); err != nil {
		t.Fatalf("SetArtistMusicBrainzID: %v", err)
	}

	_, ok, err := s.FindArtistIDByMusicBrainzID(ctx(), "mbid-solo", a.ID)
	if err != nil {
		t.Fatalf("FindArtistIDByMusicBrainzID: %v", err)
	}
	if ok {
		t.Fatal("expected no match — the only row with this mbid is the excluded one")
	}
}

func TestFindArtistIDByMusicBrainzIDNoMatch(t *testing.T) {
	s := testStore(t)
	_, ok, err := s.FindArtistIDByMusicBrainzID(ctx(), "nonexistent-mbid", 0)
	if err != nil {
		t.Fatalf("FindArtistIDByMusicBrainzID: %v", err)
	}
	if ok {
		t.Fatal("expected no match for an mbid nothing carries")
	}
}

func TestMergeArtistsReassignsAlbumsTracksAndPhotos(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Loser Artist", "Loser Album")
	insertTrackFor(t, s, "Winner Artist", "Winner Album")
	loser := findArtistByName(t, s, "Loser Artist")
	winner := findArtistByName(t, s, "Winner Artist")
	s.AddArtistPhoto(ctx(), loser.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")

	if err := s.MergeArtists(ctx(), loser.ID, winner.ID); err != nil {
		t.Fatalf("MergeArtists: %v", err)
	}

	detail, err := s.GetArtist(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("GetArtist(winner): %v", err)
	}
	if len(detail.Albums) != 2 {
		t.Fatalf("winner albums = %+v, want 2 (its own + the loser's)", detail.Albums)
	}
	if len(detail.Photos) != 1 {
		t.Fatalf("winner photos = %+v, want 1 (reassigned from loser)", detail.Photos)
	}

	if _, err := s.GetArtist(ctx(), loser.ID); !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("GetArtist(loser) error = %v, want ErrArtistNotFound", err)
	}
}

func TestMergeArtistsFoldsSameTitledAlbumTogether(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Loser Artist", "Shared Title")
	insertTrackFor(t, s, "Winner Artist", "Shared Title")
	loser := findArtistByName(t, s, "Loser Artist")
	winner := findArtistByName(t, s, "Winner Artist")

	// Both albums are titled "Shared Title", so fetch each one scoped to
	// its own artist rather than an ambiguous title lookup.
	loserBefore, err := s.GetArtist(ctx(), loser.ID)
	if err != nil {
		t.Fatalf("GetArtist(loser) before merge: %v", err)
	}
	winnerBefore, err := s.GetArtist(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("GetArtist(winner) before merge: %v", err)
	}
	winnerAlbumID := winnerBefore.Albums[0].ID
	if winnerAlbumID == loserBefore.Albums[0].ID {
		t.Fatal("test setup bug: loser and winner ended up sharing one album row before the merge even ran")
	}

	if err := s.MergeArtists(ctx(), loser.ID, winner.ID); err != nil {
		t.Fatalf("MergeArtists: %v", err)
	}

	detail, err := s.GetArtist(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("GetArtist(winner): %v", err)
	}
	if len(detail.Albums) != 1 {
		t.Fatalf("winner albums = %+v, want exactly 1 — same-titled albums must fold into one, not sit side by side", detail.Albums)
	}
	if detail.Albums[0].ID != winnerAlbumID {
		t.Fatalf("surviving album id = %d, want the winner's own album id %d (the loser's album row should be the one that disappears)", detail.Albums[0].ID, winnerAlbumID)
	}

	albumDetail, err := s.GetAlbum(ctx(), winnerAlbumID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if len(albumDetail.Tracks) != 2 {
		t.Fatalf("merged album tracks = %+v, want 2 (one from each original album)", albumDetail.Tracks)
	}
}

func TestMergeArtistsDedupesPrimaryPhoto(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Loser Artist", "Album")
	insertTrackFor(t, s, "Winner Artist", "Album 2")
	loser := findArtistByName(t, s, "Loser Artist")
	winner := findArtistByName(t, s, "Winner Artist")
	// Each gets its own first (and therefore primary) photo.
	s.AddArtistPhoto(ctx(), loser.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	s.AddArtistPhoto(ctx(), winner.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	if err := s.MergeArtists(ctx(), loser.ID, winner.ID); err != nil {
		t.Fatalf("MergeArtists: %v", err)
	}

	photos, err := s.ListArtistPhotos(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 2 {
		t.Fatalf("photos = %+v, want both photos kept (2)", photos)
	}
	primaryCount := 0
	for _, p := range photos {
		if p.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount != 1 {
		t.Fatalf("primaryCount = %d, want exactly 1 after merging two independently-primary galleries", primaryCount)
	}
}

func TestMergeArtistsRejectsSelfMerge(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Solo Artist", "Album")
	a := findArtistByName(t, s, "Solo Artist")

	if err := s.MergeArtists(ctx(), a.ID, a.ID); !errors.Is(err, ErrCannotMergeIntoSelf) {
		t.Fatalf("MergeArtists(self) error = %v, want ErrCannotMergeIntoSelf", err)
	}
}

func TestMergeArtistsNotFound(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Winner Artist", "Album")
	winner := findArtistByName(t, s, "Winner Artist")

	if err := s.MergeArtists(ctx(), 999999, winner.ID); !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("MergeArtists(nonexistent loser) error = %v, want ErrArtistNotFound", err)
	}
}

func TestFindAlbumIDByMusicBrainzIDFindsMatch(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Album A")
	insertTrackFor(t, s, "Artist", "Album B")
	a := findAlbumByTitle(t, s, "Album A")
	b := findAlbumByTitle(t, s, "Album B")

	if err := s.SetAlbumMusicBrainzID(ctx(), b.ID, "rg-shared"); err != nil {
		t.Fatalf("SetAlbumMusicBrainzID: %v", err)
	}

	id, ok, err := s.FindAlbumIDByMusicBrainzID(ctx(), "rg-shared", a.ID)
	if err != nil {
		t.Fatalf("FindAlbumIDByMusicBrainzID: %v", err)
	}
	if !ok || id != b.ID {
		t.Fatalf("id, ok = %d, %v, want %d, true", id, ok, b.ID)
	}
}

func TestMergeAlbumsReassignsTracksAndCovers(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Loser Album")
	insertTrackFor(t, s, "Artist", "Winner Album")
	loser := findAlbumByTitle(t, s, "Loser Album")
	winner := findAlbumByTitle(t, s, "Winner Album")
	s.AddAlbumCover(ctx(), loser.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")

	if err := s.MergeAlbums(ctx(), loser.ID, winner.ID); err != nil {
		t.Fatalf("MergeAlbums: %v", err)
	}

	detail, err := s.GetAlbum(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("GetAlbum(winner): %v", err)
	}
	if len(detail.Tracks) != 2 {
		t.Fatalf("winner tracks = %+v, want 2", detail.Tracks)
	}
	if len(detail.Covers) != 1 {
		t.Fatalf("winner covers = %+v, want 1 (reassigned from loser)", detail.Covers)
	}

	if _, err := s.GetAlbum(ctx(), loser.ID); !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("GetAlbum(loser) error = %v, want ErrAlbumNotFound", err)
	}
}

func TestMergeAlbumsDedupesPrimaryCover(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Loser Album")
	insertTrackFor(t, s, "Artist", "Winner Album")
	loser := findAlbumByTitle(t, s, "Loser Album")
	winner := findAlbumByTitle(t, s, "Winner Album")
	s.AddAlbumCover(ctx(), loser.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")
	s.AddAlbumCover(ctx(), winner.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")

	if err := s.MergeAlbums(ctx(), loser.ID, winner.ID); err != nil {
		t.Fatalf("MergeAlbums: %v", err)
	}

	covers, err := s.ListAlbumCovers(ctx(), winner.ID)
	if err != nil {
		t.Fatalf("ListAlbumCovers: %v", err)
	}
	if len(covers) != 2 {
		t.Fatalf("covers = %+v, want both kept (2)", covers)
	}
	primaryCount := 0
	for _, c := range covers {
		if c.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount != 1 {
		t.Fatalf("primaryCount = %d, want exactly 1", primaryCount)
	}
}

func TestMergeAlbumsRejectsDifferentArtists(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist One", "Album")
	insertTrackFor(t, s, "Artist Two", "Album")
	artistOne := findArtistByName(t, s, "Artist One")
	artistTwo := findArtistByName(t, s, "Artist Two")

	// Both albums are literally titled "Album" but under different
	// artists — fetch each one scoped to its own artist rather than an
	// ambiguous title lookup.
	oneDetail, err := s.GetArtist(ctx(), artistOne.ID)
	if err != nil {
		t.Fatalf("GetArtist(one): %v", err)
	}
	twoDetail, err := s.GetArtist(ctx(), artistTwo.ID)
	if err != nil {
		t.Fatalf("GetArtist(two): %v", err)
	}
	loserAlbum := oneDetail.Albums[0]
	winnerAlbum := twoDetail.Albums[0]

	if err := s.MergeAlbums(ctx(), loserAlbum.ID, winnerAlbum.ID); !errors.Is(err, ErrAlbumsBelongToDifferentArtists) {
		t.Fatalf("MergeAlbums(different artists) error = %v, want ErrAlbumsBelongToDifferentArtists", err)
	}
}

func TestMergeAlbumsRejectsSelfMerge(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Album")
	a := findAlbumByTitle(t, s, "Album")

	if err := s.MergeAlbums(ctx(), a.ID, a.ID); !errors.Is(err, ErrCannotMergeIntoSelf) {
		t.Fatalf("MergeAlbums(self) error = %v, want ErrCannotMergeIntoSelf", err)
	}
}

func TestMergeAlbumsNotFound(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Album")
	winner := findAlbumByTitle(t, s, "Album")

	if err := s.MergeAlbums(ctx(), 999999, winner.ID); !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("MergeAlbums(nonexistent loser) error = %v, want ErrAlbumNotFound", err)
	}
}
