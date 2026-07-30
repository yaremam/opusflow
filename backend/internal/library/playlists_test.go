package library

import (
	"errors"
	"testing"
)

// insertTrackReturningID is insertTrack plus looking its real ID back up
// (InsertTrack itself only ever returns an error — every existing test
// that needs the ID does the same round trip via ListSongs).
func insertTrackReturningID(t *testing.T, s *Store, importID int64, artist, album, title string, trackNumber, year int) int64 {
	t.Helper()
	insertTrack(t, s, importID, artist, album, title, trackNumber, year, "Pop")
	page, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10, Query: title})
	if err != nil || len(page.Items) == 0 {
		t.Fatalf("looking up inserted track %q: items=%+v err=%v", title, page.Items, err)
	}
	for _, sg := range page.Items {
		if sg.Title == title && sg.ArtistName == artist {
			return sg.ID
		}
	}
	t.Fatalf("track %q by %q not found after insert", title, artist)
	return 0
}

func TestCreatePlaylist(t *testing.T) {
	s := testStore(t)

	pl, err := s.CreatePlaylist(ctx(), "Late Night Drive")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if pl.ID == 0 {
		t.Fatalf("CreatePlaylist returned zero ID")
	}
	if pl.Name != "Late Night Drive" {
		t.Fatalf("Name = %q, want %q", pl.Name, "Late Night Drive")
	}
	if pl.TrackCount != 0 {
		t.Fatalf("TrackCount = %d, want 0 for a brand new playlist", pl.TrackCount)
	}
	if len(pl.CoverURLs) != 0 {
		t.Fatalf("CoverURLs = %+v, want empty for a playlist with no tracks", pl.CoverURLs)
	}
}

// TestGetPlaylistCoverURLsIsEmptySliceNotNil guards against a real bug: a
// nil []string marshals to JSON `null`, not `[]`, which crashed the web
// client's PlaylistCoverTile (`coverUrls.length` on null) for any playlist
// with no cover-eligible tracks — exactly the GetPlaylist path, since
// CreatePlaylist itself never went through playlistCoverURLs at all.
func TestGetPlaylistCoverURLsIsEmptySliceNotNil(t *testing.T) {
	s := testStore(t)
	created, err := s.CreatePlaylist(ctx(), "Empty")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	got, err := s.GetPlaylist(ctx(), created.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if got.CoverURLs == nil {
		t.Fatalf("CoverURLs = nil, want a non-nil empty slice (marshals to JSON null instead of [])")
	}
}

func TestListPlaylistsNewestFirstByDefault(t *testing.T) {
	s := testStore(t)
	if _, err := s.CreatePlaylist(ctx(), "First"); err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.CreatePlaylist(ctx(), "Second"); err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	page, err := s.ListPlaylists(ctx(), ListOptions{Page: 1, PageSize: 10, Sort: "recent"})
	if err != nil {
		t.Fatalf("ListPlaylists: %v", err)
	}
	if len(page.Items) != 2 || page.Items[0].Name != "Second" || page.Items[1].Name != "First" {
		t.Fatalf("playlists = %+v, want [Second, First]", page.Items)
	}
	if page.TotalCount != 2 {
		t.Fatalf("TotalCount = %d, want 2", page.TotalCount)
	}
}

func TestGetPlaylistNotFound(t *testing.T) {
	s := testStore(t)
	_, err := s.GetPlaylist(ctx(), 999999)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("GetPlaylist error = %v, want ErrPlaylistNotFound", err)
	}
}

func TestRenamePlaylist(t *testing.T) {
	s := testStore(t)
	pl, err := s.CreatePlaylist(ctx(), "Working Title")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := s.RenamePlaylist(ctx(), pl.ID, "Sunday Coffee"); err != nil {
		t.Fatalf("RenamePlaylist: %v", err)
	}

	got, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if got.Name != "Sunday Coffee" {
		t.Fatalf("Name after rename = %q, want %q", got.Name, "Sunday Coffee")
	}
}

func TestDeletePlaylistCascadesToItsTracks(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	trackID := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Temp")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, trackID); err != nil {
		t.Fatalf("AddTrackToPlaylist: %v", err)
	}

	if err := s.DeletePlaylist(ctx(), pl.ID); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}

	_, err = s.GetPlaylist(ctx(), pl.ID)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("GetPlaylist after delete: err = %v, want ErrPlaylistNotFound", err)
	}
}

func TestDeletePlaylistNotFound(t *testing.T) {
	s := testStore(t)
	err := s.DeletePlaylist(ctx(), 999999)
	if !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("DeletePlaylist error = %v, want ErrPlaylistNotFound", err)
	}
}

func TestAddTrackToPlaylistAppendsInOrder(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	track1 := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	track2 := insertTrackReturningID(t, s, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Late Night Drive")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track1); err != nil {
		t.Fatalf("AddTrackToPlaylist(track1): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track2); err != nil {
		t.Fatalf("AddTrackToPlaylist(track2): %v", err)
	}

	detail, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if detail.TrackCount != 2 {
		t.Fatalf("TrackCount = %d, want 2", detail.TrackCount)
	}
	if len(detail.Tracks) != 2 || detail.Tracks[0].Title != "Cosmic Voyager" || detail.Tracks[1].Title != "Digital Horizon" {
		t.Fatalf("Tracks = %+v, want [Cosmic Voyager, Digital Horizon] in order", detail.Tracks)
	}
}

// TestAddTrackToPlaylistAllowsDuplicates is AC-6 — matches addToQueue's
// own no-dedup rule.
func TestAddTrackToPlaylistAllowsDuplicates(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	trackID := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Repeat")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, trackID); err != nil {
		t.Fatalf("AddTrackToPlaylist (first): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, trackID); err != nil {
		t.Fatalf("AddTrackToPlaylist (second): %v", err)
	}

	detail, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(detail.Tracks) != 2 {
		t.Fatalf("Tracks = %+v, want 2 entries for the same track", detail.Tracks)
	}
	if detail.Tracks[0].PlaylistTrackID == detail.Tracks[1].PlaylistTrackID {
		t.Fatalf("both entries share PlaylistTrackID = %d, want distinct IDs so each is individually addressable", detail.Tracks[0].PlaylistTrackID)
	}
}

func TestRemovePlaylistTrackRenumbersRemainingPositions(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	track1 := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	track2 := insertTrackReturningID(t, s, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)
	track3 := insertTrackReturningID(t, s, importID, "Night Parade", "Afterglow", "Departure", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Three Tracks")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	pt1, err := s.AddTrackToPlaylist(ctx(), pl.ID, track1)
	if err != nil {
		t.Fatalf("AddTrackToPlaylist(track1): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track2); err != nil {
		t.Fatalf("AddTrackToPlaylist(track2): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track3); err != nil {
		t.Fatalf("AddTrackToPlaylist(track3): %v", err)
	}

	if err := s.RemovePlaylistTrack(ctx(), pl.ID, pt1.PlaylistTrackID); err != nil {
		t.Fatalf("RemovePlaylistTrack: %v", err)
	}

	detail, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(detail.Tracks) != 2 || detail.Tracks[0].Title != "Digital Horizon" || detail.Tracks[1].Title != "Departure" {
		t.Fatalf("Tracks after removal = %+v, want [Digital Horizon, Departure]", detail.Tracks)
	}
}

func TestReorderPlaylistTracksPersists(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	track1 := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	track2 := insertTrackReturningID(t, s, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)
	track3 := insertTrackReturningID(t, s, importID, "Night Parade", "Afterglow", "Departure", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Reorder Me")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	pt1, err := s.AddTrackToPlaylist(ctx(), pl.ID, track1)
	if err != nil {
		t.Fatalf("AddTrackToPlaylist(track1): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track2); err != nil {
		t.Fatalf("AddTrackToPlaylist(track2): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track3); err != nil {
		t.Fatalf("AddTrackToPlaylist(track3): %v", err)
	}

	// Move the first track (Cosmic Voyager) to the end.
	if err := s.ReorderPlaylistTracks(ctx(), pl.ID, pt1.PlaylistTrackID, 2); err != nil {
		t.Fatalf("ReorderPlaylistTracks: %v", err)
	}

	detail, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(detail.Tracks) != 3 ||
		detail.Tracks[0].Title != "Digital Horizon" ||
		detail.Tracks[1].Title != "Departure" ||
		detail.Tracks[2].Title != "Cosmic Voyager" {
		t.Fatalf("Tracks after reorder = %+v, want [Digital Horizon, Departure, Cosmic Voyager]", detail.Tracks)
	}
}

func TestDeletingATrackRemovesItFromPlaylists(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	trackID := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)

	pl, err := s.CreatePlaylist(ctx(), "Fragile")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, trackID); err != nil {
		t.Fatalf("AddTrackToPlaylist: %v", err)
	}

	// Deleting the track's artist cascades to the track (existing FK
	// behavior) — playlist_tracks.track_id must cascade the same way.
	page, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Cosmic Voyager"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListSongs: items=%+v err=%v", page.Items, err)
	}
	if err := s.DeleteArtist(ctx(), page.Items[0].ArtistID, false); err != nil {
		t.Fatalf("DeleteArtist: %v", err)
	}

	detail, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(detail.Tracks) != 0 {
		t.Fatalf("Tracks after the underlying track was deleted = %+v, want empty", detail.Tracks)
	}
}

func TestPlaylistCoverURLsFromFirstFourTracksAlbumArt(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	track1 := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	track2 := insertTrackReturningID(t, s, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)

	page, err := s.ListSongs(ctx(), ListOptions{Page: 1, PageSize: 10, Query: "Cosmic Voyager"})
	if err != nil || len(page.Items) != 1 {
		t.Fatalf("ListSongs: items=%+v err=%v", page.Items, err)
	}
	if _, err := s.AddAlbumCover(ctx(), page.Items[0].AlbumID, "/artwork/midnight-sun/thumb.jpg", "/artwork/midnight-sun/full.jpg", "upload", "", "hash1"); err != nil {
		t.Fatalf("AddAlbumCover: %v", err)
	}

	pl, err := s.CreatePlaylist(ctx(), "Cover Test")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track1); err != nil {
		t.Fatalf("AddTrackToPlaylist(track1): %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), pl.ID, track2); err != nil {
		t.Fatalf("AddTrackToPlaylist(track2): %v", err)
	}

	got, err := s.GetPlaylist(ctx(), pl.ID)
	if err != nil {
		t.Fatalf("GetPlaylist: %v", err)
	}
	if len(got.CoverURLs) != 2 || got.CoverURLs[0] != "/artwork/midnight-sun/thumb.jpg" || got.CoverURLs[1] != "" {
		t.Fatalf("CoverURLs = %+v, want [midnight-sun thumb, \"\"] (second track's album has no cover yet)", got.CoverURLs)
	}
}

func TestListPlaylistsContainingTrack(t *testing.T) {
	s := testStore(t)
	importID := mustCreateImport(t, s)
	track1 := insertTrackReturningID(t, s, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	track2 := insertTrackReturningID(t, s, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)

	inBoth, err := s.CreatePlaylist(ctx(), "In Both")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	notContaining, err := s.CreatePlaylist(ctx(), "Not Containing")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), inBoth.ID, track1); err != nil {
		t.Fatalf("AddTrackToPlaylist: %v", err)
	}
	if _, err := s.AddTrackToPlaylist(ctx(), notContaining.ID, track2); err != nil {
		t.Fatalf("AddTrackToPlaylist: %v", err)
	}

	got, err := s.ListPlaylistsContainingTrack(ctx(), track1)
	if err != nil {
		t.Fatalf("ListPlaylistsContainingTrack: %v", err)
	}
	if len(got) != 1 || got[0].ID != inBoth.ID {
		t.Fatalf("playlists containing track1 = %+v, want just %+v", got, inBoth)
	}
}
