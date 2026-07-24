package library

import (
	"context"
	"testing"
)

// catalogCapturingStore extends fakeDirectoryStore's directory behavior
// with catalog methods that just record the ListOptions they were called
// with, so tests can assert on what Service normalized before delegating —
// without needing a real database.
type catalogCapturingStore struct {
	*fakeDirectoryStore
	lastArtistOpts ListOptions
	lastAlbumOpts  ListOptions
	lastSongOpts   ListOptions
}

func newCatalogCapturingStore() *catalogCapturingStore {
	return &catalogCapturingStore{fakeDirectoryStore: newFakeDirectoryStore()}
}

func (c *catalogCapturingStore) ListArtists(_ context.Context, opts ListOptions) (Page[Artist], error) {
	c.lastArtistOpts = opts
	return Page[Artist]{Items: []Artist{}, Page: opts.Page, PageSize: opts.PageSize}, nil
}

func (c *catalogCapturingStore) GetArtist(_ context.Context, id int64) (ArtistDetail, error) {
	if id == 0 {
		return ArtistDetail{}, ErrArtistNotFound
	}
	return ArtistDetail{Artist: Artist{ID: id}}, nil
}

func (c *catalogCapturingStore) ListAlbums(_ context.Context, opts ListOptions) (Page[Album], error) {
	c.lastAlbumOpts = opts
	return Page[Album]{Items: []Album{}, Page: opts.Page, PageSize: opts.PageSize}, nil
}

func (c *catalogCapturingStore) GetAlbum(_ context.Context, id int64) (AlbumDetail, error) {
	if id == 0 {
		return AlbumDetail{}, ErrAlbumNotFound
	}
	return AlbumDetail{Album: Album{ID: id}}, nil
}

func (c *catalogCapturingStore) ListSongs(_ context.Context, opts ListOptions) (Page[Song], error) {
	c.lastSongOpts = opts
	return Page[Song]{Items: []Song{}, Page: opts.Page, PageSize: opts.PageSize}, nil
}

func TestListArtistsNormalizesDefaults(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(Roots{t.TempDir()}, store, newRecordingScanner())

	if _, err := svc.ListArtists(ctx(), ListOptions{}); err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	got := store.lastArtistOpts
	if got.Page != 1 {
		t.Fatalf("Page = %d, want 1", got.Page)
	}
	if got.PageSize != defaultPageSize {
		t.Fatalf("PageSize = %d, want %d", got.PageSize, defaultPageSize)
	}
	if got.Sort != "recent" {
		t.Fatalf("Sort = %q, want %q", got.Sort, "recent")
	}
}

func TestListAlbumsClampsPageAndPageSize(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(Roots{t.TempDir()}, store, newRecordingScanner())

	if _, err := svc.ListAlbums(ctx(), ListOptions{Page: -5, PageSize: 9000}); err != nil {
		t.Fatalf("ListAlbums: %v", err)
	}
	got := store.lastAlbumOpts
	if got.Page != 1 {
		t.Fatalf("Page = %d, want clamped to 1", got.Page)
	}
	if got.PageSize != maxPageSize {
		t.Fatalf("PageSize = %d, want clamped to %d", got.PageSize, maxPageSize)
	}
}

func TestListSongsRejectsUnknownSort(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(Roots{t.TempDir()}, store, newRecordingScanner())

	if _, err := svc.ListSongs(ctx(), ListOptions{Sort: "banana"}); err != nil {
		t.Fatalf("ListSongs: %v", err)
	}
	if store.lastSongOpts.Sort != "recent" {
		t.Fatalf("Sort = %q, want fallback to %q", store.lastSongOpts.Sort, "recent")
	}
}

func TestListOptionsPreservesGenreYearQuery(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(Roots{t.TempDir()}, store, newRecordingScanner())

	opts := ListOptions{Genre: "Jazz", Year: 1965, Query: "sinner", Sort: "name"}
	if _, err := svc.ListArtists(ctx(), opts); err != nil {
		t.Fatalf("ListArtists: %v", err)
	}
	got := store.lastArtistOpts
	if got.Genre != "Jazz" || got.Year != 1965 || got.Query != "sinner" || got.Sort != "name" {
		t.Fatalf("opts = %+v, want filters preserved", got)
	}
}

func TestServiceGetArtistAndAlbumDelegate(t *testing.T) {
	store := newCatalogCapturingStore()
	svc := NewService(Roots{t.TempDir()}, store, newRecordingScanner())

	artist, err := svc.GetArtist(ctx(), 42)
	if err != nil {
		t.Fatalf("GetArtist: %v", err)
	}
	if artist.ID != 42 {
		t.Fatalf("artist.ID = %d, want 42", artist.ID)
	}

	album, err := svc.GetAlbum(ctx(), 7)
	if err != nil {
		t.Fatalf("GetAlbum: %v", err)
	}
	if album.ID != 7 {
		t.Fatalf("album.ID = %d, want 7", album.ID)
	}
}
