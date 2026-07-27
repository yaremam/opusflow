package library

import (
	"errors"
	"testing"
)

func TestAddArtistPhotoCreatesFirstAsPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	p, err := s.AddArtistPhoto(ctx(), artist.ID, "/artwork/artist/1/a/thumb.jpg", "/artwork/artist/1/a/full.jpg", "upload", "hash-a")
	if err != nil {
		t.Fatalf("AddArtistPhoto: %v", err)
	}
	if !p.IsPrimary {
		t.Fatal("first photo added for an artist should be marked primary")
	}
}

func TestAddArtistPhotoSecondIsNotAutoPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	if _, err := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a"); err != nil {
		t.Fatalf("AddArtistPhoto #1: %v", err)
	}
	p2, err := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")
	if err != nil {
		t.Fatalf("AddArtistPhoto #2: %v", err)
	}
	if p2.IsPrimary {
		t.Fatal("a second photo must not silently become primary, overriding whatever's already set")
	}
}

func TestAddArtistPhotoDedupesByContentHash(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	first, err := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "same-hash")
	if err != nil {
		t.Fatalf("AddArtistPhoto #1: %v", err)
	}
	second, err := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "cover_art_archive", "same-hash")
	if err != nil {
		t.Fatalf("AddArtistPhoto #2: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("a duplicate content hash should return the existing photo (id %d), got a new one (id %d)", first.ID, second.ID)
	}

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 1 {
		t.Fatalf("len(photos) = %d, want 1 — the duplicate must not have been inserted", len(photos))
	}
}

func TestAddArtistPhotoBlankHashNeverDedupes(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	if _, err := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "legacy", ""); err != nil {
		t.Fatalf("AddArtistPhoto #1: %v", err)
	}
	if _, err := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "legacy", ""); err != nil {
		t.Fatalf("AddArtistPhoto #2: %v", err)
	}

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 2 {
		t.Fatalf("len(photos) = %d, want 2 — a blank hash (legacy rows) must never dedupe against anything", len(photos))
	}
}

func TestListArtistPhotosOrdersByAddedTime(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	first, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	second, _ := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 2 || photos[0].ID != first.ID || photos[1].ID != second.ID {
		t.Fatalf("photos = %+v, want [%d, %d] in add order", photos, first.ID, second.ID)
	}
}

func TestSetArtistPrimaryPhotoSwitchesPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	first, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	second, _ := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	if err := s.SetArtistPrimaryPhoto(ctx(), artist.ID, second.ID); err != nil {
		t.Fatalf("SetArtistPrimaryPhoto: %v", err)
	}

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	for _, p := range photos {
		wantPrimary := p.ID == second.ID
		if p.IsPrimary != wantPrimary {
			t.Fatalf("photo %d IsPrimary = %v, want %v", p.ID, p.IsPrimary, wantPrimary)
		}
	}
	_ = first
}

func TestSetArtistPrimaryPhotoNotFound(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	if err := s.SetArtistPrimaryPhoto(ctx(), artist.ID, 999999); !errors.Is(err, ErrArtistPhotoNotFound) {
		t.Fatalf("SetArtistPrimaryPhoto(nonexistent) = %v, want ErrArtistPhotoNotFound", err)
	}
}

func TestDeleteArtistPhotoRemovesRow(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	p, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	if _, _, err := s.DeleteArtistPhoto(ctx(), artist.ID, p.ID); err != nil {
		t.Fatalf("DeleteArtistPhoto: %v", err)
	}

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 0 {
		t.Fatalf("len(photos) = %d, want 0 after delete", len(photos))
	}
}

func TestDeleteArtistPhotoPromotesNewPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	first, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	second, _ := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")
	_ = second

	if _, _, err := s.DeleteArtistPhoto(ctx(), artist.ID, first.ID); err != nil {
		t.Fatalf("DeleteArtistPhoto: %v", err)
	}

	photos, err := s.ListArtistPhotos(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("ListArtistPhotos: %v", err)
	}
	if len(photos) != 1 || !photos[0].IsPrimary {
		t.Fatalf("photos = %+v, want the one remaining photo promoted to primary", photos)
	}
}

func TestDeleteArtistPhotoReturnsPathsForCallerToUnlink(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	p, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	thumb, full, err := s.DeleteArtistPhoto(ctx(), artist.ID, p.ID)
	if err != nil {
		t.Fatalf("DeleteArtistPhoto: %v", err)
	}
	if thumb != "/a/thumb.jpg" || full != "/a/full.jpg" {
		t.Fatalf("thumb/full = %q/%q, want the deleted photo's own paths", thumb, full)
	}
}

func TestDeleteArtistPhotoNotFound(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")

	if _, _, err := s.DeleteArtistPhoto(ctx(), artist.ID, 999999); !errors.Is(err, ErrArtistPhotoNotFound) {
		t.Fatalf("DeleteArtistPhoto(nonexistent) = %v, want ErrArtistPhotoNotFound", err)
	}
}

func TestGetArtistIncludesPhotoGallery(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")
	s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")

	detail, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if len(detail.Photos) != 2 {
		t.Fatalf("len(Photos) = %d, want 2", len(detail.Photos))
	}
}

func TestListArtistsShowsPrimaryPhotoAsThumbnail(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Gallery Artist", "Album")
	artist := findArtistByName(t, s, "Gallery Artist")
	first, _ := s.AddArtistPhoto(ctx(), artist.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "hash-a")
	second, _ := s.AddArtistPhoto(ctx(), artist.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "hash-b")
	_ = first
	if err := s.SetArtistPrimaryPhoto(ctx(), artist.ID, second.ID); err != nil {
		t.Fatalf("SetArtistPrimaryPhoto: %v", err)
	}

	page, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 30})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	for _, a := range page.Items {
		if a.ID == artist.ID {
			if a.PhotoThumbURL != "/b/thumb.jpg" {
				t.Fatalf("PhotoThumbURL = %q, want the primary photo's thumb (/b/thumb.jpg)", a.PhotoThumbURL)
			}
			return
		}
	}
	t.Fatal("artist not found in ListArtists results")
}

// --- Album covers: same behavior, mirrored ---

func TestAddAlbumCoverCreatesFirstAsPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")

	c, err := s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "front", "hash-a")
	if err != nil {
		t.Fatalf("AddAlbumCover: %v", err)
	}
	if !c.IsPrimary {
		t.Fatal("first cover added for an album should be marked primary")
	}
	if c.PictureType != "front" {
		t.Fatalf("PictureType = %q, want front", c.PictureType)
	}
}

func TestAddAlbumCoverDedupesByContentHash(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")

	first, err := s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "embedded", "front", "same-hash")
	if err != nil {
		t.Fatalf("AddAlbumCover #1: %v", err)
	}
	second, err := s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "cover_art_archive", "front", "same-hash")
	if err != nil {
		t.Fatalf("AddAlbumCover #2: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate content hash should return the existing cover, got a new one")
	}
}

func TestSetAlbumPrimaryCoverSwitchesPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")

	s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")
	second, _ := s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")

	if err := s.SetAlbumPrimaryCover(ctx(), album.ID, second.ID); err != nil {
		t.Fatalf("SetAlbumPrimaryCover: %v", err)
	}

	covers, err := s.ListAlbumCovers(ctx(), album.ID)
	if err != nil {
		t.Fatalf("ListAlbumCovers: %v", err)
	}
	for _, c := range covers {
		wantPrimary := c.ID == second.ID
		if c.IsPrimary != wantPrimary {
			t.Fatalf("cover %d IsPrimary = %v, want %v", c.ID, c.IsPrimary, wantPrimary)
		}
	}
}

func TestDeleteAlbumCoverPromotesNewPrimary(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")

	first, _ := s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")
	s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")

	if _, _, err := s.DeleteAlbumCover(ctx(), album.ID, first.ID); err != nil {
		t.Fatalf("DeleteAlbumCover: %v", err)
	}

	covers, err := s.ListAlbumCovers(ctx(), album.ID)
	if err != nil {
		t.Fatalf("ListAlbumCovers: %v", err)
	}
	if len(covers) != 1 || !covers[0].IsPrimary {
		t.Fatalf("covers = %+v, want the one remaining cover promoted to primary", covers)
	}
}

func TestGetAlbumIncludesCoverGallery(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")
	s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "front", "hash-a")
	s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "back", "hash-b")

	detail, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if len(detail.Covers) != 2 {
		t.Fatalf("len(Covers) = %d, want 2", len(detail.Covers))
	}
}

func TestListAlbumsShowsPrimaryCoverAsThumbnail(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")
	s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")
	second, _ := s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")
	if err := s.SetAlbumPrimaryCover(ctx(), album.ID, second.ID); err != nil {
		t.Fatalf("SetAlbumPrimaryCover: %v", err)
	}

	page, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 30})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	for _, al := range page.Items {
		if al.ID == album.ID {
			if al.CoverThumbURL != "/b/thumb.jpg" {
				t.Fatalf("CoverThumbURL = %q, want the primary cover's thumb (/b/thumb.jpg)", al.CoverThumbURL)
			}
			return
		}
	}
	t.Fatal("album not found in ListAlbums results")
}

func TestListSongsShowsAlbumPrimaryCoverAsThumbnail(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Artist", "Gallery Album")
	album := findAlbumByTitle(t, s, "Gallery Album")
	second, _ := s.AddAlbumCover(ctx(), album.ID, "/b/thumb.jpg", "/b/full.jpg", "upload", "", "hash-b")
	s.AddAlbumCover(ctx(), album.ID, "/a/thumb.jpg", "/a/full.jpg", "upload", "", "hash-a")
	if err := s.SetAlbumPrimaryCover(ctx(), album.ID, second.ID); err != nil {
		t.Fatalf("SetAlbumPrimaryCover: %v", err)
	}

	page, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 30})
	if err != nil {
		t.Fatalf("ListSongs: %v", err)
	}
	found := false
	for _, sg := range page.Items {
		if sg.AlbumID == album.ID {
			found = true
			if sg.AlbumCoverThumbURL != "/b/thumb.jpg" {
				t.Fatalf("AlbumCoverThumbURL = %q, want the primary cover's thumb", sg.AlbumCoverThumbURL)
			}
		}
	}
	if !found {
		t.Fatal("expected at least one song from the seeded album")
	}
}
