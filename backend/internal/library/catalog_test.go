package library

import (
	"errors"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/scan"
)

func insertTrack(t *testing.T, s *Store, dirID int64, artist, album, title string, trackNumber, year int, genre string) {
	t.Helper()
	if err := s.InsertTrack(ctx(), scan.Track{
		DirectoryID: dirID,
		Path:        "/music/" + artist + "/" + album + "/" + title + ".mp3",
		Title:       title,
		Artist:      artist,
		Album:       album,
		TrackNumber: trackNumber,
		Year:        year,
		Genre:       genre,
	}); err != nil {
		t.Fatalf("InsertTrack(%q, %q, %q): %v", artist, album, title, err)
	}
}

func TestInsertTrackDedupesArtistAndAlbum(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Rock")

	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "Bodysnatchers", 2, 2007, "Alternative Rock")

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 {
		t.Fatalf("artists = %+v, want exactly 1", artists.Items)
	}
	if artists.Items[0].Name != "Radiohead" || artists.Items[0].TrackCount != 2 || artists.Items[0].AlbumCount != 1 {
		t.Fatalf("artist = %+v", artists.Items[0])
	}

	albums, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums.Items) != 1 {
		t.Fatalf("albums = %+v, want exactly 1", albums.Items)
	}
	if albums.Items[0].Title != "In Rainbows" || albums.Items[0].TrackCount != 2 || albums.Items[0].ArtistName != "Radiohead" {
		t.Fatalf("album = %+v", albums.Items[0])
	}
}

func TestInsertTrackUntaggedGroupsUnderUnknown(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Misc")

	insertTrack(t, s, dir.ID, "", "", "track1", 0, 0, "")
	insertTrack(t, s, dir.ID, "", "", "track2", 0, 0, "")

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 || artists.Items[0].Name != "" || artists.Items[0].TrackCount != 2 {
		t.Fatalf("artists = %+v, want one Unknown Artist with 2 tracks", artists.Items)
	}
}

func TestListArtistsPagination(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Various")

	names := []string{"Artist A", "Artist B", "Artist C", "Artist D", "Artist E"}
	for _, name := range names {
		insertTrack(t, s, dir.ID, name, "Album", "Song", 1, 2020, "Pop")
	}

	page1, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 2, Sort: "name"})
	if err != nil {
		t.Fatalf("ListArtists page1: %v", err)
	}
	if page1.TotalCount != 5 || len(page1.Items) != 2 {
		t.Fatalf("page1 = %+v", page1)
	}
	if page1.Items[0].Name != "Artist A" || page1.Items[1].Name != "Artist B" {
		t.Fatalf("page1 items = %+v, want A, B", page1.Items)
	}

	page3, err := s.ListArtists(ctx(), ListOptions{Page: 3, PageSize: 2, Sort: "name"})
	if err != nil {
		t.Fatalf("ListArtists page3: %v", err)
	}
	if len(page3.Items) != 1 || page3.Items[0].Name != "Artist E" {
		t.Fatalf("page3 items = %+v, want just E", page3.Items)
	}
}

func TestListAlbumsFiltersByGenreAndYear(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Mixed")

	insertTrack(t, s, dir.ID, "Fleetwood Mac", "Rumours", "Dreams", 1, 1977, "Rock")
	insertTrack(t, s, dir.ID, "Tycho", "Weather", "Dye", 1, 2019, "Electronic")

	byGenre, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10, Genre: "Electronic"})
	if err != nil {
		t.Fatalf("ListAlbums genre filter: %v", err)
	}
	if len(byGenre.Items) != 1 || byGenre.Items[0].Title != "Weather" {
		t.Fatalf("genre-filtered albums = %+v, want just Weather", byGenre.Items)
	}

	byYear, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10, Year: 1977})
	if err != nil {
		t.Fatalf("ListAlbums year filter: %v", err)
	}
	if len(byYear.Items) != 1 || byYear.Items[0].Title != "Rumours" {
		t.Fatalf("year-filtered albums = %+v, want just Rumours", byYear.Items)
	}
}

func TestListAlbumsGenreFilterMatchesSubstring(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/GenreSubstring")

	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, dir.ID, "boygenius", "the record", "Cool About It", 1, 2023, "Indie Rock")
	insertTrack(t, s, dir.ID, "Tycho", "Weather", "Dye", 1, 2019, "Electronic")

	got, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10, Genre: "rock"})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(got.Items) != 2 {
		t.Fatalf("albums = %+v, want 2 (matching Alternative Rock and Indie Rock)", got.Items)
	}
}

func TestListSongsSearchMatchesTitleArtistOrAlbum(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Search")

	insertTrack(t, s, dir.ID, "Nina Simone", "Pastel Blues", "Sinnerman", 1, 1965, "Jazz")
	insertTrack(t, s, dir.ID, "Kendrick Lamar", "Mr. Morale", "Rich Spirit", 1, 2022, "Hip Hop")

	byTitle, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "sinner"})
	if err != nil {
		t.Fatalf("ListSongs title search: %v", err)
	}
	if len(byTitle.Items) != 1 || byTitle.Items[0].Title != "Sinnerman" {
		t.Fatalf("title search = %+v", byTitle.Items)
	}

	byArtist, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "kendrick"})
	if err != nil {
		t.Fatalf("ListSongs artist search: %v", err)
	}
	if len(byArtist.Items) != 1 || byArtist.Items[0].ArtistName != "Kendrick Lamar" {
		t.Fatalf("artist search = %+v", byArtist.Items)
	}
}

func TestGetArtistNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetArtist(ctx(), 999999)
	if !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("GetArtist error = %v, want ErrArtistNotFound", err)
	}
}

func TestGetArtistReturnsAlbumsNewestFirst(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/Radiohead")

	insertTrack(t, s, dir.ID, "Radiohead", "The Bends", "Fake Plastic Trees", 1, 1995, "Alternative Rock")
	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "Reckoner", 1, 2007, "Alternative Rock")

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 {
		t.Fatalf("artists = %+v", artists.Items)
	}

	detail, err := s.GetArtist(ctx(), artists.Items[0].ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if len(detail.Albums) != 2 {
		t.Fatalf("albums = %+v, want 2", detail.Albums)
	}
	if detail.Albums[0].Title != "In Rainbows" || detail.Albums[1].Title != "The Bends" {
		t.Fatalf("albums order = %+v, want In Rainbows (2007) before The Bends (1995)", detail.Albums)
	}
}

func TestGetAlbumNotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetAlbum(ctx(), 999999)
	if !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("GetAlbum error = %v, want ErrAlbumNotFound", err)
	}
}

func TestGetAlbumReturnsTracksOrderedByTrackNumber(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/InRainbows")

	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "Reckoner", 7, 2007, "Alternative Rock")
	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, dir.ID, "Radiohead", "In Rainbows", "Bodysnatchers", 2, 2007, "Alternative Rock")

	albums, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums.Items) != 1 {
		t.Fatalf("albums = %+v", albums.Items)
	}

	detail, err := s.GetAlbum(ctx(), albums.Items[0].ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if len(detail.Tracks) != 3 {
		t.Fatalf("tracks = %+v, want 3", detail.Tracks)
	}
	got := []string{detail.Tracks[0].Title, detail.Tracks[1].Title, detail.Tracks[2].Title}
	want := []string{"15 Step", "Bodysnatchers", "Reckoner"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tracks order = %v, want %v", got, want)
		}
	}
}

func TestRemoveDirectoryDeletesOrphanedArtistAndAlbum(t *testing.T) {
	s := testStore(t)
	dir, _ := s.AddDirectory(ctx(), "/music", "/music/OnlyHere")
	insertTrack(t, s, dir.ID, "Solo Artist", "Solo Album", "Only Song", 1, 2020, "Pop")

	if err := s.RemoveDirectory(ctx(), dir.ID); err != nil {
		t.Fatalf("RemoveDirectory: %v", err)
	}

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Solo Artist"})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 0 {
		t.Fatalf("artists after removal = %+v, want none (orphan should be deleted)", artists.Items)
	}

	albums, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Solo Album"})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums.Items) != 0 {
		t.Fatalf("albums after removal = %+v, want none (orphan should be deleted)", albums.Items)
	}
}

func TestRemoveDirectoryKeepsArtistStillReferencedElsewhere(t *testing.T) {
	s := testStore(t)
	dirA, _ := s.AddDirectory(ctx(), "/music", "/music/DirA")
	dirB, _ := s.AddDirectory(ctx(), "/music", "/music/DirB")

	insertTrack(t, s, dirA.ID, "Shared Artist", "Album One", "Song One", 1, 2020, "Pop")
	insertTrack(t, s, dirB.ID, "Shared Artist", "Album Two", "Song Two", 1, 2021, "Pop")

	if err := s.RemoveDirectory(ctx(), dirA.ID); err != nil {
		t.Fatalf("RemoveDirectory: %v", err)
	}

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Shared Artist"})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 || artists.Items[0].TrackCount != 1 || artists.Items[0].AlbumCount != 1 {
		t.Fatalf("artists after partial removal = %+v, want 1 artist with 1 remaining track/album", artists.Items)
	}
}
