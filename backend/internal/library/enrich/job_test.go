package enrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeStore is an in-memory Store, recording every Set* call so tests can
// assert exactly what the Job wrote (and, just as importantly, what it
// left untouched — AC-8's independent-status guarantee).
type fakeStore struct {
	artists []ArtistTarget
	albums  []AlbumTarget

	artistMBIDCalls []int64
	artistArtCalls  map[int64]struct {
		status            Status
		thumbURL, fullURL string
	}
	artistFactsCalls map[int64]struct {
		status Status
		info   ArtistInfo
	}
	artistBioCalls map[int64]struct {
		status         Status
		bio, sourceURL string
	}

	albumMBIDCalls []int64
	albumArtCalls  map[int64]struct {
		status            Status
		thumbURL, fullURL string
	}
	albumFactsCalls map[int64]struct {
		status Status
		info   ReleaseGroupInfo
	}
	albumDescriptionCalls map[int64]struct {
		status      Status
		description string
		sourceURL   string
	}
	albumCoverCalls []addedAlbumCover

	// artistsByMBID/albumsByMBID let a test simulate "another row already
	// has this musicbrainz id" (TDR 017) without a real relational store —
	// set directly by the test, read by FindArtistIDByMusicBrainzID/
	// FindAlbumIDByMusicBrainzID.
	artistsByMBID map[string]int64
	albumsByMBID  map[string]int64

	mergeArtistCalls []mergeCall
	mergeAlbumCalls  []mergeCall
}

type mergeCall struct{ loserID, winnerID int64 }

// addedAlbumCover records one AddAlbumCoverForEnrichment call — used by
// Cover Art Archive tests (TDR 014 Stage 2) to assert every image Job
// found got added, not just one.
type addedAlbumCover struct {
	albumID                            int64
	thumbURL, fullURL, source, picType string
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		artistArtCalls: map[int64]struct {
			status            Status
			thumbURL, fullURL string
		}{},
		artistFactsCalls: map[int64]struct {
			status Status
			info   ArtistInfo
		}{},
		artistBioCalls: map[int64]struct {
			status         Status
			bio, sourceURL string
		}{},
		albumArtCalls: map[int64]struct {
			status            Status
			thumbURL, fullURL string
		}{},
		albumFactsCalls: map[int64]struct {
			status Status
			info   ReleaseGroupInfo
		}{},
		albumDescriptionCalls: map[int64]struct {
			status      Status
			description string
			sourceURL   string
		}{},
		artistsByMBID: map[string]int64{},
		albumsByMBID:  map[string]int64{},
	}
}

func (f *fakeStore) ArtistsNeedingEnrichment(context.Context, int) ([]ArtistTarget, error) {
	return f.artists, nil
}
func (f *fakeStore) AlbumsNeedingEnrichment(context.Context, int) ([]AlbumTarget, error) {
	return f.albums, nil
}
func (f *fakeStore) SetArtistMusicBrainzID(_ context.Context, id int64, mbid string) error {
	f.artistMBIDCalls = append(f.artistMBIDCalls, id)
	return nil
}
func (f *fakeStore) SetArtistArt(_ context.Context, id int64, status Status, thumbURL, fullURL string) error {
	f.artistArtCalls[id] = struct {
		status            Status
		thumbURL, fullURL string
	}{status, thumbURL, fullURL}
	return nil
}
func (f *fakeStore) SetArtistFacts(_ context.Context, id int64, status Status, info ArtistInfo) error {
	f.artistFactsCalls[id] = struct {
		status Status
		info   ArtistInfo
	}{status, info}
	return nil
}
func (f *fakeStore) SetArtistBio(_ context.Context, id int64, status Status, bio, sourceURL string) error {
	f.artistBioCalls[id] = struct {
		status         Status
		bio, sourceURL string
	}{status, bio, sourceURL}
	return nil
}
func (f *fakeStore) SetAlbumMusicBrainzID(_ context.Context, id int64, mbid string) error {
	f.albumMBIDCalls = append(f.albumMBIDCalls, id)
	return nil
}
func (f *fakeStore) SetAlbumArt(_ context.Context, id int64, status Status, thumbURL, fullURL string) error {
	f.albumArtCalls[id] = struct {
		status            Status
		thumbURL, fullURL string
	}{status, thumbURL, fullURL}
	return nil
}
func (f *fakeStore) SetAlbumFacts(_ context.Context, id int64, status Status, info ReleaseGroupInfo) error {
	f.albumFactsCalls[id] = struct {
		status Status
		info   ReleaseGroupInfo
	}{status, info}
	return nil
}
func (f *fakeStore) SetAlbumDescription(_ context.Context, id int64, status Status, description, sourceURL string) error {
	f.albumDescriptionCalls[id] = struct {
		status      Status
		description string
		sourceURL   string
	}{status, description, sourceURL}
	return nil
}
func (f *fakeStore) AddAlbumCoverForEnrichment(_ context.Context, id int64, thumbURL, fullURL, source, pictureType, contentHash string) error {
	f.albumCoverCalls = append(f.albumCoverCalls, addedAlbumCover{id, thumbURL, fullURL, source, pictureType})
	return nil
}
func (f *fakeStore) FindArtistIDByMusicBrainzID(_ context.Context, mbid string, excludeID int64) (int64, bool, error) {
	id, ok := f.artistsByMBID[mbid]
	if !ok || id == excludeID {
		return 0, false, nil
	}
	return id, true, nil
}
func (f *fakeStore) MergeArtists(_ context.Context, loserID, winnerID int64) error {
	f.mergeArtistCalls = append(f.mergeArtistCalls, mergeCall{loserID, winnerID})
	return nil
}
func (f *fakeStore) FindAlbumIDByMusicBrainzID(_ context.Context, mbid string, excludeID int64) (int64, bool, error) {
	id, ok := f.albumsByMBID[mbid]
	if !ok || id == excludeID {
		return 0, false, nil
	}
	return id, true, nil
}
func (f *fakeStore) MergeAlbums(_ context.Context, loserID, winnerID int64) error {
	f.mergeAlbumCalls = append(f.mergeAlbumCalls, mergeCall{loserID, winnerID})
	return nil
}

// testClients wires a Job's three network clients against one combined
// httptest server, routed by path prefix, plus a real ImageStore backed by
// a temp dir.
type testClients struct {
	mb     *MusicBrainz
	caa    *CoverArtArchive
	wd     *Wikidata
	images *ImageStore
}

func newTestClients(t *testing.T, mux *http.ServeMux) *testClients {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mb := NewMusicBrainz("opusflow-test/0.1")
	mb.baseURL = srv.URL + "/mb"
	mb.limiter = newRateLimiter(0)

	caa := NewCoverArtArchive("opusflow-test/0.1")
	caa.baseURL = srv.URL + "/caa"

	wd := NewWikidata("opusflow-test/0.1")
	wd.apiBaseURL = srv.URL + "/wikidata"
	wd.commonsBaseURL = srv.URL + "/commons"
	wd.wikipediaBaseURL = srv.URL + "/wikipedia"

	return &testClients{mb: mb, caa: caa, wd: wd, images: NewImageStore(t.TempDir())}
}

func TestJobResolvesArtistArtFactsAndBioEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": [{"id": "artist-mbid"}]}`))
	})
	mux.HandleFunc("/mb/artist/artist-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"life-span": {"begin": "2011"}, "area": {"name": "UK"},
			"genres": [{"name": "Dream pop"}],
			"relations": [{"type": "wikidata", "url": {"resource": "https://www.wikidata.org/wiki/Q1"}}]
		}`))
	})
	mux.HandleFunc("/wikidata", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"entities": {"Q1": {
			"sitelinks": {"enwiki": {"title": "Marlow Creek"}},
			"claims": {"P18": [{"mainsnak": {"datavalue": {"value": "photo.jpg"}}}]}
		}}}`))
	})
	mux.HandleFunc("/wikipedia/Marlow%20Creek", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"extract": "A band.", "content_urls": {"desktop": {"page": "https://en.wikipedia.org/wiki/Marlow_Creek"}}}`))
	})
	mux.HandleFunc("/commons/photo.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Write(testPNG(t, 400, 400))
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artists = []ArtistTarget{{ID: 1, Name: "Marlow Creek", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	job := NewJob(store, clients.mb, clients.caa, clients.wd, clients.images)
	job.Run(context.Background())

	if len(store.artistMBIDCalls) != 1 || store.artistMBIDCalls[0] != 1 {
		t.Fatalf("expected MBID cached for artist 1, got %+v", store.artistMBIDCalls)
	}
	facts := store.artistFactsCalls[1]
	if facts.status != Found || facts.info.FormedYear != 2011 || facts.info.Country != "UK" {
		t.Fatalf("facts = %+v", facts)
	}
	bio := store.artistBioCalls[1]
	if bio.status != Found || bio.bio != "A band." || bio.sourceURL == "" {
		t.Fatalf("bio = %+v", bio)
	}
	art := store.artistArtCalls[1]
	if art.status != Found || art.thumbURL == "" || art.fullURL == "" {
		t.Fatalf("art = %+v", art)
	}
}

func TestJobMarksAllKindsNotFoundWhenArtistSearchHasNoMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": []}`))
	})
	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artists = []ArtistTarget{{ID: 5, Name: "Nobody", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	sum := NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if store.artistFactsCalls[5].status != NotFound {
		t.Fatalf("facts status = %v, want not_found", store.artistFactsCalls[5].status)
	}
	if store.artistBioCalls[5].status != NotFound {
		t.Fatalf("bio status = %v, want not_found", store.artistBioCalls[5].status)
	}
	if store.artistArtCalls[5].status != NotFound {
		t.Fatalf("art status = %v, want not_found", store.artistArtCalls[5].status)
	}
	if len(store.artistMBIDCalls) != 0 {
		t.Fatalf("expected no MBID cached for an unmatched artist, got %+v", store.artistMBIDCalls)
	}
	if sum != (RunSummary{NotFound: 3}) {
		t.Fatalf("Run summary = %+v, want {NotFound: 3}", sum)
	}
}

func TestJobRunSummaryCountsFound(t *testing.T) {
	// Reuses the same fixtures as the end-to-end resolution test above —
	// all three kinds resolve to Found — to check Run's returned tally,
	// not just what got written to the store.
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": [{"id": "artist-mbid"}]}`))
	})
	mux.HandleFunc("/mb/artist/artist-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"life-span": {"begin": "2011"}, "area": {"name": "UK"},
			"genres": [{"name": "Dream pop"}],
			"relations": [{"type": "wikidata", "url": {"resource": "https://www.wikidata.org/wiki/Q1"}}]
		}`))
	})
	mux.HandleFunc("/wikidata", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"entities": {"Q1": {
			"sitelinks": {"enwiki": {"title": "Marlow Creek"}},
			"claims": {"P18": [{"mainsnak": {"datavalue": {"value": "photo.jpg"}}}]}
		}}}`))
	})
	mux.HandleFunc("/wikipedia/Marlow%20Creek", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"extract": "A band.", "content_urls": {"desktop": {"page": "https://en.wikipedia.org/wiki/Marlow_Creek"}}}`))
	})
	mux.HandleFunc("/commons/photo.jpg", func(w http.ResponseWriter, r *http.Request) {
		w.Write(testPNG(t, 400, 400))
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artists = []ArtistTarget{{ID: 1, Name: "Marlow Creek", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	sum := NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if sum != (RunSummary{Found: 3}) {
		t.Fatalf("Run summary = %+v, want {Found: 3}", sum)
	}
}

func TestJobRunSummaryCountsFailed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artists = []ArtistTarget{{ID: 8, Name: "Broken Lookup", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	sum := NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if sum != (RunSummary{Failed: 3}) {
		t.Fatalf("Run summary = %+v, want {Failed: 3}", sum)
	}
}

func TestJobSkipsAlreadyFoundKindsAndReusesCachedMBID(t *testing.T) {
	var artistSearchHits, artistLookupHits int
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		artistSearchHits++
		w.Write([]byte(`{"artists": []}`))
	})
	mux.HandleFunc("/mb/artist/cached-mbid", func(w http.ResponseWriter, r *http.Request) {
		artistLookupHits++
		w.Write([]byte(`{"life-span": {}, "area": {}, "genres": [], "relations": []}`))
	})
	clients := newTestClients(t, mux)
	store := newFakeStore()
	// Facts and art already found by a prior run; only bio is still
	// pending. MusicBrainzID is already cached, so the search endpoint
	// must not be hit again.
	store.artists = []ArtistTarget{{
		ID: 9, Name: "Settled Artist", MusicBrainzID: "cached-mbid",
		ArtStatus: Found, FactsStatus: Found, BioStatus: Pending,
	}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if artistSearchHits != 0 {
		t.Fatalf("expected cached MBID to skip the search call, got %d hits", artistSearchHits)
	}
	if artistLookupHits != 1 {
		t.Fatalf("expected exactly one lookup call, got %d", artistLookupHits)
	}
	if _, wrote := store.artistFactsCalls[9]; wrote {
		t.Fatal("expected already-found facts to be left untouched")
	}
	if _, wrote := store.artistArtCalls[9]; wrote {
		t.Fatal("expected already-found art to be left untouched")
	}
	if store.artistBioCalls[9].status != NotFound {
		t.Fatalf("expected bio to be resolved (no wikidata relation) to not_found, got %+v", store.artistBioCalls[9])
	}
}

// TestJobMergesDuplicateArtistWhenCurrentRowIsTheLoser is TDR 017 AC-1/2:
// when the artist Job is currently processing turns out to share a
// freshly-resolved MusicBrainz ID with an existing, lower-ID row, Job
// merges the current (higher-ID) row into the existing one and stops —
// no further facts/bio/art calls should land against the row that no
// longer exists.
func TestJobMergesDuplicateArtistWhenCurrentRowIsTheLoser(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": [{"id": "shared-mbid"}]}`))
	})
	mux.HandleFunc("/mb/artist/shared-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"life-span": {}, "area": {}, "genres": [], "relations": []}`))
	})
	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artistsByMBID["shared-mbid"] = 2 // an existing row already carries this mbid
	store.artists = []ArtistTarget{{ID: 5, Name: "Duplicate Artist", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if len(store.mergeArtistCalls) != 1 {
		t.Fatalf("mergeArtistCalls = %+v, want exactly 1", store.mergeArtistCalls)
	}
	if got := store.mergeArtistCalls[0]; got.loserID != 5 || got.winnerID != 2 {
		t.Fatalf("merge call = %+v, want {loserID:5 winnerID:2} (lower id wins)", got)
	}
	if _, wrote := store.artistFactsCalls[5]; wrote {
		t.Fatal("expected no further facts write against the merged-away row")
	}
	if _, wrote := store.artistBioCalls[5]; wrote {
		t.Fatal("expected no further bio write against the merged-away row")
	}
	if _, wrote := store.artistArtCalls[5]; wrote {
		t.Fatal("expected no further art write against the merged-away row")
	}
}

// TestJobMergesDuplicateArtistWhenCurrentRowIsTheWinner is the mirror
// case: the row Job is processing has the lower ID, so it survives the
// merge and processing continues normally afterward.
func TestJobMergesDuplicateArtistWhenCurrentRowIsTheWinner(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/artist", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"artists": [{"id": "shared-mbid"}]}`))
	})
	mux.HandleFunc("/mb/artist/shared-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"life-span": {"begin": "1999"}, "area": {}, "genres": [], "relations": []}`))
	})
	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.artistsByMBID["shared-mbid"] = 9 // an existing row with a higher id — it's the loser
	store.artists = []ArtistTarget{{ID: 1, Name: "Canonical Artist", ArtStatus: Pending, FactsStatus: Pending, BioStatus: Pending}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if len(store.mergeArtistCalls) != 1 {
		t.Fatalf("mergeArtistCalls = %+v, want exactly 1", store.mergeArtistCalls)
	}
	if got := store.mergeArtistCalls[0]; got.loserID != 9 || got.winnerID != 1 {
		t.Fatalf("merge call = %+v, want {loserID:9 winnerID:1}", got)
	}
	if store.artistFactsCalls[1].status != Found || store.artistFactsCalls[1].info.FormedYear != 1999 {
		t.Fatalf("expected the surviving row to still be enriched normally, got %+v", store.artistFactsCalls[1])
	}
}

// TestJobMergesDuplicateAlbumSharingReleaseGroupMBID is TDR 017 AC-5, the
// album-flavored counterpart of the artist merge above.
func TestJobMergesDuplicateAlbumSharingReleaseGroupMBID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/release-group", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"release-groups": [{"id": "shared-rg-mbid"}]}`))
	})
	mux.HandleFunc("/caa/shared-rg-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": []}`))
	})
	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.albumsByMBID["shared-rg-mbid"] = 3
	store.albums = []AlbumTarget{{ID: 8, Title: "Duplicate Album", ArtistName: "Some Artist", ArtStatus: Pending, FactsStatus: Pending, DescriptionStatus: Pending}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if len(store.mergeAlbumCalls) != 1 {
		t.Fatalf("mergeAlbumCalls = %+v, want exactly 1", store.mergeAlbumCalls)
	}
	if got := store.mergeAlbumCalls[0]; got.loserID != 8 || got.winnerID != 3 {
		t.Fatalf("merge call = %+v, want {loserID:8 winnerID:3}", got)
	}
	if _, wrote := store.albumArtCalls[8]; wrote {
		t.Fatal("expected no further art write against the merged-away album")
	}
}

func TestJobResolvesAlbumArtFromCoverArtArchiveIndependentlyOfFacts(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/release-group", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"release-groups": [{"id": "rg-mbid"}]}`))
	})
	mux.HandleFunc("/caa/release-group/rg-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": [{"types": ["Front"], "image": "http://` + r.Host + `/caa/front.png"}]}`))
	})
	mux.HandleFunc("/caa/front.png", func(w http.ResponseWriter, r *http.Request) {
		w.Write(testPNG(t, 500, 500))
	})
	mux.HandleFunc("/mb/release-group/rg-mbid", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"genres": [], "relations": [], "releases": []}`))
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.albums = []AlbumTarget{{
		ID: 3, Title: "Night Vessels", ArtistName: "Marlow Creek",
		ArtStatus: Pending, FactsStatus: Pending, DescriptionStatus: Pending,
	}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	art := store.albumArtCalls[3]
	if art.status != Found {
		t.Fatalf("art = %+v", art)
	}
	if len(store.albumCoverCalls) != 1 || store.albumCoverCalls[0].thumbURL == "" ||
		store.albumCoverCalls[0].source != "cover_art_archive" || store.albumCoverCalls[0].picType != "front" {
		t.Fatalf("albumCoverCalls = %+v, want one front cover added from cover_art_archive", store.albumCoverCalls)
	}
	// No releases in the release-group lookup response -> no facts, and no
	// wikidata relation -> no description. Both distinct not_found
	// outcomes, independent of art having succeeded.
	if store.albumFactsCalls[3].status != NotFound {
		t.Fatalf("facts status = %v", store.albumFactsCalls[3].status)
	}
	if store.albumDescriptionCalls[3].status != NotFound {
		t.Fatalf("description status = %v", store.albumDescriptionCalls[3].status)
	}
}

func TestJobAddsEveryCoverArtArchiveImageNotJustFront(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/release-group", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"release-groups": [{"id": "rg-multi"}]}`))
	})
	mux.HandleFunc("/caa/release-group/rg-multi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"images": [
			{"types": ["Front"], "image": "http://` + r.Host + `/caa/front.png"},
			{"types": ["Back"], "image": "http://` + r.Host + `/caa/back.png"},
			{"types": ["Booklet"], "image": "http://` + r.Host + `/caa/booklet.png"}
		]}`))
	})
	mux.HandleFunc("/caa/front.png", func(w http.ResponseWriter, r *http.Request) { w.Write(testPNG(t, 500, 500)) })
	mux.HandleFunc("/caa/back.png", func(w http.ResponseWriter, r *http.Request) { w.Write(testPNG(t, 400, 400)) })
	mux.HandleFunc("/caa/booklet.png", func(w http.ResponseWriter, r *http.Request) { w.Write(testPNG(t, 300, 300)) })
	mux.HandleFunc("/mb/release-group/rg-multi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"genres": [], "relations": [], "releases": []}`))
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.albums = []AlbumTarget{{
		ID: 9, Title: "Triptych", ArtistName: "Someone",
		ArtStatus: Pending, FactsStatus: Found, DescriptionStatus: Found,
	}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if store.albumArtCalls[9].status != Found {
		t.Fatalf("art status = %v, want found", store.albumArtCalls[9].status)
	}
	if len(store.albumCoverCalls) != 3 {
		t.Fatalf("albumCoverCalls = %+v, want 3 (front, back, booklet all added)", store.albumCoverCalls)
	}
	gotTypes := map[string]bool{}
	for _, c := range store.albumCoverCalls {
		gotTypes[c.picType] = true
	}
	for _, want := range []string{"front", "back", "booklet"} {
		if !gotTypes[want] {
			t.Fatalf("albumCoverCalls = %+v, missing picture type %q", store.albumCoverCalls, want)
		}
	}
}

func TestJobMarksAlbumArtNotFoundWhenCoverArtArchiveHasNothing(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/mb/release-group", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"release-groups": [{"id": "rg-mbid-2"}]}`))
	})
	mux.HandleFunc("/caa/release-group/rg-mbid-2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/mb/release-group/rg-mbid-2", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"genres": [], "relations": [], "releases": []}`))
	})

	clients := newTestClients(t, mux)
	store := newFakeStore()
	store.albums = []AlbumTarget{{ID: 7, Title: "Untitled", ArtistName: "Someone", ArtStatus: Pending, FactsStatus: Found, DescriptionStatus: Found}}

	NewJob(store, clients.mb, clients.caa, clients.wd, clients.images).Run(context.Background())

	if store.albumArtCalls[7].status != NotFound {
		t.Fatalf("art status = %v, want not_found", store.albumArtCalls[7].status)
	}
	if len(store.albumCoverCalls) != 0 {
		t.Fatalf("albumCoverCalls = %+v, want none", store.albumCoverCalls)
	}
	if _, wrote := store.albumFactsCalls[7]; wrote {
		t.Fatal("expected already-found facts to be left untouched")
	}
	if _, wrote := store.albumDescriptionCalls[7]; wrote {
		t.Fatal("expected already-found description to be left untouched")
	}
}
