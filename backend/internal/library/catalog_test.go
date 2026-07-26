package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library/enrich"
	"github.com/yaremam/opusflow/backend/internal/library/organize"
)

func insertTrack(t *testing.T, s *Store, importID int64, artist, album, title string, trackNumber, year int, genre string) {
	t.Helper()
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{
		ImportID:    importID,
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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "Bodysnatchers", 2, 2007, "Alternative Rock")

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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "", "", "track1", 0, 0, "")
	insertTrack(t, s, importID, "", "", "track2", 0, 0, "")

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
	importID := mustCreateImport(t, s)

	names := []string{"Artist A", "Artist B", "Artist C", "Artist D", "Artist E"}
	for _, name := range names {
		insertTrack(t, s, importID, name, "Album", "Song", 1, 2020, "Pop")
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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Fleetwood Mac", "Rumours", "Dreams", 1, 1977, "Rock")
	insertTrack(t, s, importID, "Tycho", "Weather", "Dye", 1, 2019, "Electronic")

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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, importID, "boygenius", "the record", "Cool About It", 1, 2023, "Indie Rock")
	insertTrack(t, s, importID, "Tycho", "Weather", "Dye", 1, 2019, "Electronic")

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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Nina Simone", "Pastel Blues", "Sinnerman", 1, 1965, "Jazz")
	insertTrack(t, s, importID, "Kendrick Lamar", "Mr. Morale", "Rich Spirit", 1, 2022, "Hip Hop")

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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Radiohead", "The Bends", "Fake Plastic Trees", 1, 1995, "Alternative Rock")
	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "Reckoner", 1, 2007, "Alternative Rock")

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

// TestArtStatusExposedOnFreshAndEnrichedRows locks in TDR 007's AC-1: the
// API-facing Artist/Album (and, via embedding, ArtistDetail/AlbumDetail)
// must carry ArtStatus, defaulting to pending and reflecting a later
// SetArtistArt/SetAlbumArt write — not just the derived photo/cover URL.
func TestArtStatusExposedOnFreshAndEnrichedRows(t *testing.T) {
	s := testStore(t)
	insertTrackFor(t, s, "Status Artist", "Status Album")
	artist := findArtistByName(t, s, "Status Artist")
	album := findAlbumByTitle(t, s, "Status Album")

	if artist.ArtStatus != enrich.Pending {
		t.Fatalf("fresh artist ArtStatus = %q, want pending", artist.ArtStatus)
	}
	if album.ArtStatus != enrich.Pending {
		t.Fatalf("fresh album ArtStatus = %q, want pending", album.ArtStatus)
	}

	if err := s.SetArtistArt(ctx(), artist.ID, enrich.Found, "/artwork/artist/1/thumb.jpg", "/artwork/artist/1/full.jpg"); err != nil {
		t.Fatalf("SetArtistArt: %v", err)
	}
	if err := s.SetAlbumArt(ctx(), album.ID, enrich.NotFound, "", ""); err != nil {
		t.Fatalf("SetAlbumArt: %v", err)
	}

	gotArtist, err := s.GetArtist(ctx(), artist.ID)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if gotArtist.ArtStatus != enrich.Found {
		t.Fatalf("GetArtist ArtStatus = %q, want found", gotArtist.ArtStatus)
	}

	gotAlbum, err := s.GetAlbum(ctx(), album.ID)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if gotAlbum.ArtStatus != enrich.NotFound {
		t.Fatalf("GetAlbum ArtStatus = %q, want not_found", gotAlbum.ArtStatus)
	}

	songs, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSongs: %v", err)
	}
	found := false
	for _, sg := range songs.Items {
		if sg.AlbumID == album.ID {
			found = true
			if sg.AlbumArtStatus != enrich.NotFound {
				t.Fatalf("song AlbumArtStatus = %q, want not_found", sg.AlbumArtStatus)
			}
		}
	}
	if !found {
		t.Fatal("expected the inserted song in ListSongs")
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
	importID := mustCreateImport(t, s)

	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "Reckoner", 7, 2007, "Alternative Rock")
	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "15 Step", 1, 2007, "Alternative Rock")
	insertTrack(t, s, importID, "Radiohead", "In Rainbows", "Bodysnatchers", 2, 2007, "Alternative Rock")

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

func TestDeleteArtistRemovesAlbumsAndTracks(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	insertTrack(t, s, importID, "Solo Artist", "Solo Album", "Only Song", 1, 2020, "Pop")

	artist := findArtistByName(t, s, "Solo Artist")
	if err := s.DeleteArtist(ctx(), artist.ID, false); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}

	if _, err := s.GetArtist(ctx(), artist.ID); !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("GetArtist after delete = %v, want ErrArtistNotFound", err)
	}
	albums, err := s.ListAlbums(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Solo Album"})
	if err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	if len(albums.Items) != 0 {
		t.Fatalf("albums after artist delete = %+v, want none", albums.Items)
	}
}

func TestDeleteArtistNotFound(t *testing.T) {
	s := testStore(t)

	err := s.DeleteArtist(ctx(), 999999, false)
	if !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("DeleteArtist error = %v, want ErrArtistNotFound", err)
	}
}

func TestDeleteArtistKeepsFilesWhenNotRequested(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	importID := mustCreateImport(t, s)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: importID, Path: file, Title: "Song", Artist: "File Artist", Album: "File Album"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	artist := findArtistByName(t, s, "File Artist")
	if err := s.DeleteArtist(ctx(), artist.ID, false); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("expected file to remain on disk, stat error: %v", err)
	}
}

func TestDeleteArtistDeletesFilesWhenRequested(t *testing.T) {
	s := testStore(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	importID := mustCreateImport(t, s)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: importID, Path: file, Title: "Song", Artist: "File Artist 2", Album: "File Album 2"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	artist := findArtistByName(t, s, "File Artist 2")
	if err := s.DeleteArtist(ctx(), artist.ID, true); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}

	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be removed from disk, stat error: %v", err)
	}
}

// TestDeleteArtistDeletesFilesWhenRequestedRemovesEmptyDirectories guards
// GitHub issue #7: deleting an artist with deleteFiles removed the track
// files but left the now-empty <Artist>/<Year>.<Album>/ and <Artist>/
// directories organize's canonical layout creates around them.
func TestDeleteArtistDeletesFilesWhenRequestedRemovesEmptyDirectories(t *testing.T) {
	s := testStore(t)
	libRoot := t.TempDir()
	albumDir := filepath.Join(libRoot, "Dir Cleanup Artist", "2000.Dir Cleanup Album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(albumDir, "01.Song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	importID := mustCreateImport(t, s)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: importID, Path: file, Title: "Song", Artist: "Dir Cleanup Artist", Album: "Dir Cleanup Album"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	artist := findArtistByName(t, s, "Dir Cleanup Artist")
	if err := s.DeleteArtist(ctx(), artist.ID, true); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}

	if _, err := os.Stat(albumDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected album directory %s to be removed, stat error: %v", albumDir, err)
	}
	artistDir := filepath.Dir(albumDir)
	if _, err := os.Stat(artistDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected artist directory %s to be removed, stat error: %v", artistDir, err)
	}
	if _, err := os.Stat(libRoot); err != nil {
		t.Fatalf("expected library root %s to survive, stat error: %v", libRoot, err)
	}
}

// TestDeleteAlbumDeletesFilesWhenRequestedKeepsArtistDirWithOtherAlbums
// guards the same cleanup for DeleteAlbum, and confirms it never removes
// the artist directory while a sibling album is still there.
func TestDeleteAlbumDeletesFilesWhenRequestedKeepsArtistDirWithOtherAlbums(t *testing.T) {
	s := testStore(t)
	libRoot := t.TempDir()
	artistDir := filepath.Join(libRoot, "Multi Album Artist")
	albumDir := filepath.Join(artistDir, "2000.Removed Album")
	otherAlbumDir := filepath.Join(artistDir, "2001.Kept Album")
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherAlbumDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(albumDir, "01.Song.mp3")
	if err := os.WriteFile(file, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	otherFile := filepath.Join(otherAlbumDir, "01.Song.mp3")
	if err := os.WriteFile(otherFile, []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	importID := mustCreateImport(t, s)
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: importID, Path: file, Title: "Song", Artist: "Multi Album Artist", Album: "Removed Album"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}
	if err := s.InsertTrack(ctx(), organize.CopiedTrack{ImportID: importID, Path: otherFile, Title: "Song", Artist: "Multi Album Artist", Album: "Kept Album"}); err != nil {
		t.Fatalf("InsertTrack: %v", err)
	}

	album := findAlbumByTitle(t, s, "Removed Album")
	if err := s.DeleteAlbum(ctx(), album.ID, true); err != nil {
		t.Fatalf("DeleteAlbum: %v", err)
	}

	if _, err := os.Stat(albumDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected removed album directory %s to be gone, stat error: %v", albumDir, err)
	}
	if _, err := os.Stat(artistDir); err != nil {
		t.Fatalf("expected artist directory %s to survive (other album still there), stat error: %v", artistDir, err)
	}
	if _, err := os.Stat(otherAlbumDir); err != nil {
		t.Fatalf("expected other album directory %s to survive, stat error: %v", otherAlbumDir, err)
	}
}

func TestDeleteAlbumKeepsArtistAndOtherAlbums(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	insertTrack(t, s, importID, "Shared Artist", "Album One", "Song One", 1, 2020, "Pop")
	insertTrack(t, s, importID, "Shared Artist", "Album Two", "Song Two", 1, 2021, "Pop")

	albumTwo := findAlbumByTitle(t, s, "Album Two")
	if err := s.DeleteAlbum(ctx(), albumTwo.ID, false); err != nil {
		t.Fatalf("DeleteAlbum: %v", err)
	}

	artists, err := s.ListArtists(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Shared Artist"})
	if err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	if len(artists.Items) != 1 || artists.Items[0].TrackCount != 1 || artists.Items[0].AlbumCount != 1 {
		t.Fatalf("artists after album delete = %+v, want 1 artist with 1 remaining track/album", artists.Items)
	}
}

func TestDeleteAlbumNotFound(t *testing.T) {
	s := testStore(t)

	err := s.DeleteAlbum(ctx(), 999999, false)
	if !errors.Is(err, ErrAlbumNotFound) {
		t.Fatalf("DeleteAlbum error = %v, want ErrAlbumNotFound", err)
	}
}
