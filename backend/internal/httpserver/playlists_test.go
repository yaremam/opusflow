package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/yaremam/opusflow/backend/internal/library"
)

// songID looks up a track's real ID via the same list endpoint the app
// itself uses — mirroring how catalog_test.go's own tests already do
// this rather than reading the store's return value directly, since
// InsertTrack itself only ever returns an error.
func songID(t *testing.T, handler http.Handler, title string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/library/songs?q="+url.QueryEscape(title), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/library/songs?q=%s status = %d, body = %s", title, rec.Code, rec.Body.String())
	}
	var page library.Page[library.Song]
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("songs matching %q = %+v, want exactly one", title, page.Items)
	}
	return page.Items[0].ID
}

func TestCreateListGetRenameDeletePlaylistEndToEnd(t *testing.T) {
	store, svc := testStoreAndService(t)
	handler := New("", "", "", "", svc, nil)
	_ = store

	createReq := httptest.NewRequest(http.MethodPost, "/api/playlists", strings.NewReader(`{"name":"Late Night Drive"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/playlists status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created library.Playlist
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Name != "Late Night Drive" {
		t.Fatalf("Name = %q, want %q", created.Name, "Late Night Drive")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/playlists", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/playlists status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var list library.Page[library.Playlist]
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].ID != created.ID {
		t.Fatalf("list = %+v, want one row matching id %d", list.Items, created.ID)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/playlists/"+strconv.FormatInt(created.ID, 10), nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /api/playlists/{id} status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	var detail library.PlaylistDetail
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(detail.Tracks) != 0 {
		t.Fatalf("Tracks = %+v, want empty for a brand new playlist", detail.Tracks)
	}

	renameReq := httptest.NewRequest(http.MethodPatch, "/api/playlists/"+strconv.FormatInt(created.ID, 10), strings.NewReader(`{"name":"Sunday Coffee"}`))
	renameRec := httptest.NewRecorder()
	handler.ServeHTTP(renameRec, renameReq)
	if renameRec.Code != http.StatusOK {
		t.Fatalf("PATCH /api/playlists/{id} status = %d, body = %s", renameRec.Code, renameRec.Body.String())
	}
	var renamed library.PlaylistDetail
	if err := json.Unmarshal(renameRec.Body.Bytes(), &renamed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if renamed.Name != "Sunday Coffee" {
		t.Fatalf("Name after rename = %q, want %q", renamed.Name, "Sunday Coffee")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/playlists/"+strconv.FormatInt(created.ID, 10), nil)
	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/playlists/{id} status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	getAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/playlists/"+strconv.FormatInt(created.ID, 10), nil)
	getAfterDeleteRec := httptest.NewRecorder()
	handler.ServeHTTP(getAfterDeleteRec, getAfterDeleteReq)
	if getAfterDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/playlists/{id} after delete: status = %d, want 404", getAfterDeleteRec.Code)
	}
}

func TestAddRemoveAndReorderPlaylistTracksEndToEnd(t *testing.T) {
	store, svc := testStoreAndService(t)
	handler := New("", "", "", "", svc, nil)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	mustInsertTrack(t, store, importID, "SynthWave", "Neon Pulse", "Digital Horizon", 1, 2026)
	track1 := songID(t, handler, "Cosmic Voyager")
	track2 := songID(t, handler, "Digital Horizon")

	createReq := httptest.NewRequest(http.MethodPost, "/api/playlists", strings.NewReader(`{"name":"Two Tracks"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	var pl library.Playlist
	if err := json.Unmarshal(createRec.Body.Bytes(), &pl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	addReq1 := httptest.NewRequest(http.MethodPost, "/api/playlists/"+strconv.FormatInt(pl.ID, 10)+"/tracks", strings.NewReader(`{"trackId":`+strconv.FormatInt(track1, 10)+`}`))
	addRec1 := httptest.NewRecorder()
	handler.ServeHTTP(addRec1, addReq1)
	if addRec1.Code != http.StatusCreated {
		t.Fatalf("POST .../tracks (track1) status = %d, body = %s", addRec1.Code, addRec1.Body.String())
	}
	var pt1 library.PlaylistTrack
	if err := json.Unmarshal(addRec1.Body.Bytes(), &pt1); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	addReq2 := httptest.NewRequest(http.MethodPost, "/api/playlists/"+strconv.FormatInt(pl.ID, 10)+"/tracks", strings.NewReader(`{"trackId":`+strconv.FormatInt(track2, 10)+`}`))
	addRec2 := httptest.NewRecorder()
	handler.ServeHTTP(addRec2, addReq2)
	if addRec2.Code != http.StatusCreated {
		t.Fatalf("POST .../tracks (track2) status = %d, body = %s", addRec2.Code, addRec2.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/playlists/"+strconv.FormatInt(pl.ID, 10), nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	var detail library.PlaylistDetail
	if err := json.Unmarshal(getRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(detail.Tracks) != 2 || detail.Tracks[0].Title != "Cosmic Voyager" || detail.Tracks[1].Title != "Digital Horizon" {
		t.Fatalf("Tracks = %+v, want [Cosmic Voyager, Digital Horizon]", detail.Tracks)
	}

	// Reorder: move track1 to the end.
	reorderReq := httptest.NewRequest(http.MethodPatch, "/api/playlists/"+strconv.FormatInt(pl.ID, 10)+"/tracks/reorder",
		strings.NewReader(`{"playlistTrackId":`+strconv.FormatInt(pt1.PlaylistTrackID, 10)+`,"toIndex":1}`))
	reorderRec := httptest.NewRecorder()
	handler.ServeHTTP(reorderRec, reorderReq)
	if reorderRec.Code != http.StatusOK {
		t.Fatalf("PATCH .../tracks/reorder status = %d, body = %s", reorderRec.Code, reorderRec.Body.String())
	}
	var reordered library.PlaylistDetail
	if err := json.Unmarshal(reorderRec.Body.Bytes(), &reordered); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(reordered.Tracks) != 2 || reordered.Tracks[0].Title != "Digital Horizon" || reordered.Tracks[1].Title != "Cosmic Voyager" {
		t.Fatalf("Tracks after reorder = %+v, want [Digital Horizon, Cosmic Voyager]", reordered.Tracks)
	}

	// Remove track1 (still addressable by its original PlaylistTrackID
	// even after reordering) — the response is the fresh detail, the
	// same mutate-then-return-detail shape rename/reorder already use,
	// since a removed track can shift CoverURLs.
	removeReq := httptest.NewRequest(http.MethodDelete, "/api/playlists/"+strconv.FormatInt(pl.ID, 10)+"/tracks/"+strconv.FormatInt(pt1.PlaylistTrackID, 10), nil)
	removeRec := httptest.NewRecorder()
	handler.ServeHTTP(removeRec, removeReq)
	if removeRec.Code != http.StatusOK {
		t.Fatalf("DELETE .../tracks/{playlistTrackId} status = %d, body = %s", removeRec.Code, removeRec.Body.String())
	}
	var final library.PlaylistDetail
	if err := json.Unmarshal(removeRec.Body.Bytes(), &final); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(final.Tracks) != 1 || final.Tracks[0].Title != "Digital Horizon" {
		t.Fatalf("Tracks after removal = %+v, want just [Digital Horizon]", final.Tracks)
	}
}

// TestListPlaylistsContainingTrackEndToEnd is AC-5's picker pre-check
// data source.
func TestListPlaylistsContainingTrackEndToEnd(t *testing.T) {
	store, svc := testStoreAndService(t)
	handler := New("", "", "", "", svc, nil)
	importID := mustCreateImportForTest(t, store)
	mustInsertTrack(t, store, importID, "Solaris", "Midnight Sun", "Cosmic Voyager", 1, 2026)
	trackID := songID(t, handler, "Cosmic Voyager")

	createReq := httptest.NewRequest(http.MethodPost, "/api/playlists", strings.NewReader(`{"name":"Contains It"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	var pl library.Playlist
	if err := json.Unmarshal(createRec.Body.Bytes(), &pl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	addReq := httptest.NewRequest(http.MethodPost, "/api/playlists/"+strconv.FormatInt(pl.ID, 10)+"/tracks", strings.NewReader(`{"trackId":`+strconv.FormatInt(trackID, 10)+`}`))
	addRec := httptest.NewRecorder()
	handler.ServeHTTP(addRec, addReq)
	if addRec.Code != http.StatusCreated {
		t.Fatalf("POST .../tracks status = %d, body = %s", addRec.Code, addRec.Body.String())
	}

	membershipReq := httptest.NewRequest(http.MethodGet, "/api/library/songs/"+strconv.FormatInt(trackID, 10)+"/playlists", nil)
	membershipRec := httptest.NewRecorder()
	handler.ServeHTTP(membershipRec, membershipReq)
	if membershipRec.Code != http.StatusOK {
		t.Fatalf("GET .../playlists status = %d, body = %s", membershipRec.Code, membershipRec.Body.String())
	}
	var containing []library.Playlist
	if err := json.Unmarshal(membershipRec.Body.Bytes(), &containing); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(containing) != 1 || containing[0].ID != pl.ID {
		t.Fatalf("playlists containing track = %+v, want just %+v", containing, pl)
	}
}

func TestGetPlaylistNotFoundEndToEnd(t *testing.T) {
	_, svc := testStoreAndService(t)
	handler := New("", "", "", "", svc, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/playlists/999999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/playlists/999999 status = %d, want 404", rec.Code)
	}
}
