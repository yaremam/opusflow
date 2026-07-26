package enrich

import (
	"context"
	"fmt"
	"log"
)

// maxPerRun bounds how many artists/albums a single Run processes. A
// household library fits comfortably under this in one run; anything left
// over simply waits for the next run (Run is triggered after every scan
// and once at startup — see TDR 003 §3) rather than needing to fully drain
// in one pass. Bounding it also keeps a single Run's worst case (every
// item failing) finite rather than looping.
const maxPerRun = 500

// Job is the background enrichment worker: for each artist/album still
// missing art, facts, or bio/description, it resolves a MusicBrainz match
// (caching the MBID for reuse across kinds and future runs) and attempts
// each still-pending kind independently, recording found/not_found/failed
// per kind rather than all-or-nothing (AC-8).
type Job struct {
	store  Store
	mb     *MusicBrainz
	caa    *CoverArtArchive
	wd     *Wikidata
	images *ImageStore
}

// NewJob builds a Job. store is typically *library.Store, which satisfies
// Store structurally.
func NewJob(store Store, mb *MusicBrainz, caa *CoverArtArchive, wd *Wikidata, images *ImageStore) *Job {
	return &Job{store: store, mb: mb, caa: caa, wd: wd, images: images}
}

// RunSummary tallies one Run's outcomes across every kind (art, facts,
// bio/description) it touched, for every artist and album processed —
// Run's callers log it, since "how did the last run go" would otherwise
// only be answerable by grepping this package's log lines. Distinct from
// Summary, the Wikipedia-extract type wikidata.go returns.
type RunSummary struct {
	Found    int
	NotFound int
	Failed   int
}

func (s *RunSummary) add(status Status) {
	switch status {
	case Found:
		s.Found++
	case NotFound:
		s.NotFound++
	case Failed:
		s.Failed++
	}
}

func (s *RunSummary) merge(other RunSummary) {
	s.Found += other.Found
	s.NotFound += other.NotFound
	s.Failed += other.Failed
}

// Run processes every artist, then every album, still owed at least one of
// art/facts/bio (AC-4: not scoped to any particular scan, which is what
// lets the same Run double as backfill for pre-existing libraries). It
// runs unattended in a background goroutine, so per-item errors are logged
// and recorded as that item's status rather than propagated — one bad
// lookup never aborts the rest of the run, matching scan.Scanner's
// per-file error tolerance. ctx must be independent of any single scan's
// lifecycle: Run covers the whole library, not one directory, so a caller
// must never pass a context that a directory-scoped operation (e.g. scan
// cancellation) can cancel out from under the rest of the run.
func (j *Job) Run(ctx context.Context) RunSummary {
	var total RunSummary

	artists, err := j.store.ArtistsNeedingEnrichment(ctx, maxPerRun)
	if err != nil {
		log.Printf("enrich: listing artists needing enrichment: %v", err)
	}
	for _, a := range artists {
		if ctx.Err() != nil {
			return total
		}
		total.merge(j.processArtist(ctx, a))
	}

	albums, err := j.store.AlbumsNeedingEnrichment(ctx, maxPerRun)
	if err != nil {
		log.Printf("enrich: listing albums needing enrichment: %v", err)
	}
	for _, al := range albums {
		if ctx.Err() != nil {
			return total
		}
		total.merge(j.processAlbum(ctx, al))
	}

	return total
}

func needsWork(status Status) bool {
	return status == Pending || status == Failed
}

// settle writes status for one kind if it's still open (pending or
// failed); a kind some earlier run already settled (found/not_found) is
// left untouched. label identifies the write for the log line a failed
// write produces. Returns whether it actually wrote, and what — the
// shared bookkeeping every process*/mark* call site needs to feed its
// RunSummary.
func settle(ctx context.Context, currentStatus, status Status, label string, write func(context.Context, Status) error) (applied bool, appliedStatus Status) {
	if !needsWork(currentStatus) {
		return false, ""
	}
	if err := write(ctx, status); err != nil {
		log.Printf("enrich: %s: %v", label, err)
		return false, ""
	}
	return true, status
}

// resolveKind is settle, but the status isn't fixed — it's computed by
// calling fetch: found with no error -> Found, no error but nothing
// found -> NotFound, error -> Failed. This is the shape every art/facts/
// bio resolution in this package shares; what varies per kind is only
// fetch (what to look up, and what side data — a URL, a struct — it
// captures for write to persist) and write (which Store setter to call).
func resolveKind(ctx context.Context, currentStatus Status, label string, fetch func() (found bool, err error), write func(context.Context, Status) error) (bool, Status) {
	if !needsWork(currentStatus) {
		return false, ""
	}
	found, err := fetch()
	status := Found
	switch {
	case err != nil:
		status = Failed
	case !found:
		status = NotFound
	}
	return settle(ctx, currentStatus, status, label, write)
}

// processArtist resolves a's MusicBrainz match (if not already cached) and
// attempts whichever of art/facts/bio are still pending/failed.
func (j *Job) processArtist(ctx context.Context, a ArtistTarget) RunSummary {
	mbid := a.MusicBrainzID
	if mbid == "" {
		found, ok, err := j.mb.SearchArtist(ctx, a.Name)
		if err != nil {
			return j.markArtistUnresolved(ctx, a, Failed)
		}
		if !ok {
			return j.markArtistUnresolved(ctx, a, NotFound)
		}
		mbid = found
		if err := j.store.SetArtistMusicBrainzID(ctx, a.ID, mbid); err != nil {
			log.Printf("enrich: artist %d: caching musicbrainz id: %v", a.ID, err)
		}
	}

	info, err := j.mb.LookupArtist(ctx, mbid)
	if err != nil {
		return j.markArtistUnresolved(ctx, a, Failed)
	}

	var sum RunSummary

	if _, status := resolveKind(ctx, a.FactsStatus, fmt.Sprintf("artist %d: setting facts", a.ID),
		func() (bool, error) {
			return info.FormedYear != 0 || info.Country != "" || len(info.Genres) > 0, nil
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetArtistFacts(ctx, a.ID, status, info)
		},
	); status != "" {
		sum.add(status)
	}

	var entity Entity
	if info.WikidataURL != "" {
		entity, err = j.wd.ResolveEntity(ctx, info.WikidataURL)
		if err != nil {
			log.Printf("enrich: artist %d: resolving wikidata entity: %v", a.ID, err)
		}
	}

	var summary Summary
	if _, status := resolveKind(ctx, a.BioStatus, fmt.Sprintf("artist %d: setting bio", a.ID),
		func() (bool, error) {
			if entity.WikipediaTitle == "" {
				return false, nil
			}
			var err error
			summary, err = j.wd.FetchSummary(ctx, entity.WikipediaTitle)
			return summary.Extract != "", err
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetArtistBio(ctx, a.ID, status, summary.Extract, summary.URL)
		},
	); status != "" {
		sum.add(status)
	}

	var thumbURL, fullURL string
	if _, status := resolveKind(ctx, a.ArtStatus, fmt.Sprintf("artist %d: setting art", a.ID),
		func() (bool, error) {
			if entity.ImageFilename == "" {
				return false, nil
			}
			data, err := j.wd.FetchImage(ctx, entity.ImageFilename)
			if err != nil {
				return false, err
			}
			var saveErr error
			thumbURL, fullURL, saveErr = j.images.Save("artist", a.ID, data)
			if saveErr != nil {
				log.Printf("enrich: artist %d: saving photo: %v", a.ID, saveErr)
				return false, saveErr
			}
			return true, nil
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetArtistArt(ctx, a.ID, status, thumbURL, fullURL)
		},
	); status != "" {
		sum.add(status)
	}

	return sum
}

// markArtistUnresolved records status against every still-pending/failed
// kind for an artist whose MusicBrainz match itself couldn't be
// established (search failed, or found nothing at all).
func (j *Job) markArtistUnresolved(ctx context.Context, a ArtistTarget, status Status) RunSummary {
	var sum RunSummary
	if _, s := settle(ctx, a.FactsStatus, status, fmt.Sprintf("artist %d: setting facts status", a.ID),
		func(ctx context.Context, s Status) error { return j.store.SetArtistFacts(ctx, a.ID, s, ArtistInfo{}) },
	); s != "" {
		sum.add(s)
	}
	if _, s := settle(ctx, a.BioStatus, status, fmt.Sprintf("artist %d: setting bio status", a.ID),
		func(ctx context.Context, s Status) error { return j.store.SetArtistBio(ctx, a.ID, s, "", "") },
	); s != "" {
		sum.add(s)
	}
	if _, s := settle(ctx, a.ArtStatus, status, fmt.Sprintf("artist %d: setting art status", a.ID),
		func(ctx context.Context, s Status) error { return j.store.SetArtistArt(ctx, a.ID, s, "", "") },
	); s != "" {
		sum.add(s)
	}
	return sum
}

// processAlbum resolves al's MusicBrainz match (if not already cached) and
// attempts whichever of art/facts/description are still pending/failed.
// Art comes straight from Cover Art Archive via the release-group MBID —
// unlike artist photos, it needs no Wikidata hop.
func (j *Job) processAlbum(ctx context.Context, al AlbumTarget) RunSummary {
	mbid := al.MusicBrainzID
	if mbid == "" {
		found, ok, err := j.mb.SearchReleaseGroup(ctx, al.Title, al.ArtistName)
		if err != nil {
			return j.markAlbumUnresolved(ctx, al, Failed)
		}
		if !ok {
			return j.markAlbumUnresolved(ctx, al, NotFound)
		}
		mbid = found
		if err := j.store.SetAlbumMusicBrainzID(ctx, al.ID, mbid); err != nil {
			log.Printf("enrich: album %d: caching musicbrainz id: %v", al.ID, err)
		}
	}

	var sum RunSummary

	var artThumbURL, artFullURL string
	if _, status := resolveKind(ctx, al.ArtStatus, fmt.Sprintf("album %d: setting art", al.ID),
		func() (bool, error) {
			data, found, err := j.caa.FetchFront(ctx, mbid)
			if err != nil {
				return false, err
			}
			if !found {
				return false, nil
			}
			var saveErr error
			artThumbURL, artFullURL, saveErr = j.images.Save("album", al.ID, data)
			if saveErr != nil {
				log.Printf("enrich: album %d: saving cover: %v", al.ID, saveErr)
				return false, saveErr
			}
			return true, nil
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetAlbumArt(ctx, al.ID, status, artThumbURL, artFullURL)
		},
	); status != "" {
		sum.add(status)
	}

	if !needsWork(al.FactsStatus) && !needsWork(al.DescriptionStatus) {
		return sum
	}

	info, err := j.mb.LookupReleaseGroup(ctx, mbid)
	if err != nil {
		if _, s := settle(ctx, al.FactsStatus, Failed, fmt.Sprintf("album %d: setting facts status", al.ID),
			func(ctx context.Context, s Status) error {
				return j.store.SetAlbumFacts(ctx, al.ID, s, ReleaseGroupInfo{})
			},
		); s != "" {
			sum.add(s)
		}
		if _, s := settle(ctx, al.DescriptionStatus, Failed, fmt.Sprintf("album %d: setting description status", al.ID),
			func(ctx context.Context, s Status) error { return j.store.SetAlbumDescription(ctx, al.ID, s, "", "") },
		); s != "" {
			sum.add(s)
		}
		return sum
	}

	if _, status := resolveKind(ctx, al.FactsStatus, fmt.Sprintf("album %d: setting facts", al.ID),
		func() (bool, error) {
			return info.Label != "" || info.Country != "" || len(info.Genres) > 0, nil
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetAlbumFacts(ctx, al.ID, status, info)
		},
	); status != "" {
		sum.add(status)
	}

	var summary Summary
	if _, status := resolveKind(ctx, al.DescriptionStatus, fmt.Sprintf("album %d: setting description", al.ID),
		func() (bool, error) {
			if info.WikidataURL == "" {
				return false, nil
			}
			entity, err := j.wd.ResolveEntity(ctx, info.WikidataURL)
			if err != nil {
				return false, err
			}
			if entity.WikipediaTitle == "" {
				return false, nil
			}
			summary, err = j.wd.FetchSummary(ctx, entity.WikipediaTitle)
			return summary.Extract != "", err
		},
		func(ctx context.Context, status Status) error {
			return j.store.SetAlbumDescription(ctx, al.ID, status, summary.Extract, summary.URL)
		},
	); status != "" {
		sum.add(status)
	}

	return sum
}

// markAlbumUnresolved is markArtistUnresolved's album-flavored counterpart.
func (j *Job) markAlbumUnresolved(ctx context.Context, al AlbumTarget, status Status) RunSummary {
	var sum RunSummary
	if _, s := settle(ctx, al.ArtStatus, status, fmt.Sprintf("album %d: setting art status", al.ID),
		func(ctx context.Context, s Status) error { return j.store.SetAlbumArt(ctx, al.ID, s, "", "") },
	); s != "" {
		sum.add(s)
	}
	if _, s := settle(ctx, al.FactsStatus, status, fmt.Sprintf("album %d: setting facts status", al.ID),
		func(ctx context.Context, s Status) error {
			return j.store.SetAlbumFacts(ctx, al.ID, s, ReleaseGroupInfo{})
		},
	); s != "" {
		sum.add(s)
	}
	if _, s := settle(ctx, al.DescriptionStatus, status, fmt.Sprintf("album %d: setting description status", al.ID),
		func(ctx context.Context, s Status) error { return j.store.SetAlbumDescription(ctx, al.ID, s, "", "") },
	); s != "" {
		sum.add(s)
	}
	return sum
}
