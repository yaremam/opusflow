package enrich

import (
	"context"
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

// Run processes every artist, then every album, still owed at least one of
// art/facts/bio (AC-4: not scoped to any particular scan, which is what
// lets the same Run double as backfill for pre-existing libraries). It
// runs unattended in a background goroutine, so per-item errors are logged
// and recorded as that item's status rather than propagated — one bad
// lookup never aborts the rest of the run, matching scan.Scanner's
// per-file error tolerance.
func (j *Job) Run(ctx context.Context) {
	artists, err := j.store.ArtistsNeedingEnrichment(ctx, maxPerRun)
	if err != nil {
		log.Printf("enrich: listing artists needing enrichment: %v", err)
	}
	for _, a := range artists {
		if ctx.Err() != nil {
			return
		}
		j.processArtist(ctx, a)
	}

	albums, err := j.store.AlbumsNeedingEnrichment(ctx, maxPerRun)
	if err != nil {
		log.Printf("enrich: listing albums needing enrichment: %v", err)
	}
	for _, al := range albums {
		if ctx.Err() != nil {
			return
		}
		j.processAlbum(ctx, al)
	}
}

func needsWork(status Status) bool {
	return status == Pending || status == Failed
}

// processArtist resolves a's MusicBrainz match (if not already cached) and
// attempts whichever of art/facts/bio are still pending/failed.
func (j *Job) processArtist(ctx context.Context, a ArtistTarget) {
	mbid := a.MusicBrainzID
	if mbid == "" {
		found, ok, err := j.mb.SearchArtist(ctx, a.Name)
		if err != nil {
			j.markArtistUnresolved(ctx, a, Failed)
			return
		}
		if !ok {
			j.markArtistUnresolved(ctx, a, NotFound)
			return
		}
		mbid = found
		if err := j.store.SetArtistMusicBrainzID(ctx, a.ID, mbid); err != nil {
			log.Printf("enrich: artist %d: caching musicbrainz id: %v", a.ID, err)
		}
	}

	info, err := j.mb.LookupArtist(ctx, mbid)
	if err != nil {
		j.markArtistUnresolved(ctx, a, Failed)
		return
	}

	if needsWork(a.FactsStatus) {
		if info.FormedYear != 0 || info.Country != "" || len(info.Genres) > 0 {
			j.setArtistFacts(a.ID, Found, info)
		} else {
			j.setArtistFacts(a.ID, NotFound, info)
		}
	}

	var entity Entity
	if info.WikidataURL != "" {
		entity, err = j.wd.ResolveEntity(ctx, info.WikidataURL)
		if err != nil {
			log.Printf("enrich: artist %d: resolving wikidata entity: %v", a.ID, err)
		}
	}

	if needsWork(a.BioStatus) {
		j.resolveArtistBio(ctx, a.ID, entity)
	}
	if needsWork(a.ArtStatus) {
		j.resolveArtistArt(ctx, a.ID, entity)
	}
}

// markArtistUnresolved records status against every still-pending/failed
// kind for an artist whose MusicBrainz match itself couldn't be
// established (search failed, or found nothing at all).
func (j *Job) markArtistUnresolved(ctx context.Context, a ArtistTarget, status Status) {
	if needsWork(a.FactsStatus) {
		if err := j.store.SetArtistFacts(ctx, a.ID, status, 0, "", nil); err != nil {
			log.Printf("enrich: artist %d: setting facts status: %v", a.ID, err)
		}
	}
	if needsWork(a.BioStatus) {
		if err := j.store.SetArtistBio(ctx, a.ID, status, "", ""); err != nil {
			log.Printf("enrich: artist %d: setting bio status: %v", a.ID, err)
		}
	}
	if needsWork(a.ArtStatus) {
		if err := j.store.SetArtistArt(ctx, a.ID, status, "", ""); err != nil {
			log.Printf("enrich: artist %d: setting art status: %v", a.ID, err)
		}
	}
}

func (j *Job) setArtistFacts(id int64, status Status, info ArtistInfo) {
	if err := j.store.SetArtistFacts(context.Background(), id, status, info.FormedYear, info.Country, info.Genres); err != nil {
		log.Printf("enrich: artist %d: setting facts: %v", id, err)
	}
}

func (j *Job) resolveArtistBio(ctx context.Context, id int64, entity Entity) {
	if entity.WikipediaTitle == "" {
		j.setArtistBio(ctx, id, NotFound, Summary{})
		return
	}
	summary, err := j.wd.FetchSummary(ctx, entity.WikipediaTitle)
	if err != nil {
		j.setArtistBio(ctx, id, Failed, Summary{})
		return
	}
	if summary.Extract == "" {
		j.setArtistBio(ctx, id, NotFound, Summary{})
		return
	}
	j.setArtistBio(ctx, id, Found, summary)
}

func (j *Job) setArtistBio(ctx context.Context, id int64, status Status, summary Summary) {
	if err := j.store.SetArtistBio(ctx, id, status, summary.Extract, summary.URL); err != nil {
		log.Printf("enrich: artist %d: setting bio: %v", id, err)
	}
}

func (j *Job) resolveArtistArt(ctx context.Context, id int64, entity Entity) {
	if entity.ImageFilename == "" {
		j.setArtistArt(ctx, id, NotFound, "", "")
		return
	}
	data, err := j.wd.FetchImage(ctx, entity.ImageFilename)
	if err != nil {
		j.setArtistArt(ctx, id, Failed, "", "")
		return
	}
	thumbURL, fullURL, err := j.images.Save("artist", id, data)
	if err != nil {
		log.Printf("enrich: artist %d: saving photo: %v", id, err)
		j.setArtistArt(ctx, id, Failed, "", "")
		return
	}
	j.setArtistArt(ctx, id, Found, thumbURL, fullURL)
}

func (j *Job) setArtistArt(ctx context.Context, id int64, status Status, thumbURL, fullURL string) {
	if err := j.store.SetArtistArt(ctx, id, status, thumbURL, fullURL); err != nil {
		log.Printf("enrich: artist %d: setting art: %v", id, err)
	}
}

// processAlbum resolves al's MusicBrainz match (if not already cached) and
// attempts whichever of art/facts/description are still pending/failed.
// Art comes straight from Cover Art Archive via the release-group MBID —
// unlike artist photos, it needs no Wikidata hop.
func (j *Job) processAlbum(ctx context.Context, al AlbumTarget) {
	mbid := al.MusicBrainzID
	if mbid == "" {
		found, ok, err := j.mb.SearchReleaseGroup(ctx, al.Title, al.ArtistName)
		if err != nil {
			j.markAlbumUnresolved(ctx, al, Failed)
			return
		}
		if !ok {
			j.markAlbumUnresolved(ctx, al, NotFound)
			return
		}
		mbid = found
		if err := j.store.SetAlbumMusicBrainzID(ctx, al.ID, mbid); err != nil {
			log.Printf("enrich: album %d: caching musicbrainz id: %v", al.ID, err)
		}
	}

	if needsWork(al.ArtStatus) {
		j.resolveAlbumArt(ctx, al.ID, mbid)
	}

	if needsWork(al.FactsStatus) || needsWork(al.DescriptionStatus) {
		info, err := j.mb.LookupReleaseGroup(ctx, mbid)
		if err != nil {
			if needsWork(al.FactsStatus) {
				j.setAlbumFacts(ctx, al.ID, Failed, ReleaseGroupInfo{})
			}
			if needsWork(al.DescriptionStatus) {
				j.setAlbumDescription(ctx, al.ID, Failed, Summary{})
			}
			return
		}

		if needsWork(al.FactsStatus) {
			if info.Label != "" || info.Country != "" || len(info.Genres) > 0 {
				j.setAlbumFacts(ctx, al.ID, Found, info)
			} else {
				j.setAlbumFacts(ctx, al.ID, NotFound, info)
			}
		}

		if needsWork(al.DescriptionStatus) {
			j.resolveAlbumDescription(ctx, al.ID, info.WikidataURL)
		}
	}
}

func (j *Job) markAlbumUnresolved(ctx context.Context, al AlbumTarget, status Status) {
	if needsWork(al.ArtStatus) {
		if err := j.store.SetAlbumArt(ctx, al.ID, status, "", ""); err != nil {
			log.Printf("enrich: album %d: setting art status: %v", al.ID, err)
		}
	}
	if needsWork(al.FactsStatus) {
		if err := j.store.SetAlbumFacts(ctx, al.ID, status, "", "", nil); err != nil {
			log.Printf("enrich: album %d: setting facts status: %v", al.ID, err)
		}
	}
	if needsWork(al.DescriptionStatus) {
		if err := j.store.SetAlbumDescription(ctx, al.ID, status, "", ""); err != nil {
			log.Printf("enrich: album %d: setting description status: %v", al.ID, err)
		}
	}
}

func (j *Job) resolveAlbumArt(ctx context.Context, id int64, releaseGroupMBID string) {
	data, found, err := j.caa.FetchFront(ctx, releaseGroupMBID)
	if err != nil {
		j.setAlbumArt(ctx, id, Failed, "", "")
		return
	}
	if !found {
		j.setAlbumArt(ctx, id, NotFound, "", "")
		return
	}
	thumbURL, fullURL, err := j.images.Save("album", id, data)
	if err != nil {
		log.Printf("enrich: album %d: saving cover: %v", id, err)
		j.setAlbumArt(ctx, id, Failed, "", "")
		return
	}
	j.setAlbumArt(ctx, id, Found, thumbURL, fullURL)
}

func (j *Job) setAlbumArt(ctx context.Context, id int64, status Status, thumbURL, fullURL string) {
	if err := j.store.SetAlbumArt(ctx, id, status, thumbURL, fullURL); err != nil {
		log.Printf("enrich: album %d: setting art: %v", id, err)
	}
}

func (j *Job) setAlbumFacts(ctx context.Context, id int64, status Status, info ReleaseGroupInfo) {
	if err := j.store.SetAlbumFacts(ctx, id, status, info.Label, info.Country, info.Genres); err != nil {
		log.Printf("enrich: album %d: setting facts: %v", id, err)
	}
}

func (j *Job) resolveAlbumDescription(ctx context.Context, id int64, wikidataURL string) {
	if wikidataURL == "" {
		j.setAlbumDescription(ctx, id, NotFound, Summary{})
		return
	}
	entity, err := j.wd.ResolveEntity(ctx, wikidataURL)
	if err != nil {
		j.setAlbumDescription(ctx, id, Failed, Summary{})
		return
	}
	if entity.WikipediaTitle == "" {
		j.setAlbumDescription(ctx, id, NotFound, Summary{})
		return
	}
	summary, err := j.wd.FetchSummary(ctx, entity.WikipediaTitle)
	if err != nil {
		j.setAlbumDescription(ctx, id, Failed, Summary{})
		return
	}
	if summary.Extract == "" {
		j.setAlbumDescription(ctx, id, NotFound, Summary{})
		return
	}
	j.setAlbumDescription(ctx, id, Found, summary)
}

func (j *Job) setAlbumDescription(ctx context.Context, id int64, status Status, summary Summary) {
	if err := j.store.SetAlbumDescription(ctx, id, status, summary.Extract, summary.URL); err != nil {
		log.Printf("enrich: album %d: setting description: %v", id, err)
	}
}
