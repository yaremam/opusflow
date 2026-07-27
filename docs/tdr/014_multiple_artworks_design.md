# TDR 014: Multiple Artworks per Artist/Album

## 1. Context & Architectural Requirements

GitHub issue #14 asked for multiple images per artist/album, "and a way to
display all." Grilling this converged on a real gallery (several images
shown together), not a "pick your favorite candidate" picker — and, at
every fork, on full parity across every available source rather than a
smaller first cut: manual multi-upload, Cover Art Archive's complete typed
image set, and every embedded picture across all five supported audio
formats.

Research into the current system found it's single-image-per-entity at
every layer, with no existing multi-row capability to extend:

- **Schema**: `artists.photo_thumb_path`/`photo_path` and
  `albums.cover_thumb_path`/`cover_path` (migration
  `0003_artwork_and_info.sql`) are plain scalar columns, one image slot
  each. `art_status` (an `enrich_status` enum: pending/found/not_found/
  failed) is what actually drives the background job's "does this still
  need enrichment" scheduling (`Store.ArtistsNeedingEnrichment`/
  `AlbumsNeedingEnrichment`) — that scheduling concern is orthogonal to
  how many images exist and stays untouched by this TDR.
- **Writes are hard overwrite-in-place**: `SetArtistArt`/`SetAlbumArt`
  (`enrich_store.go:94-106,171-183`) `UPDATE` the same row's path columns
  on every `Found` write; `ImageStore.Save` (`enrich/imagestore.go:51-71`)
  writes to a fixed `<dir>/<kind>/<id>/{thumb,full}.jpg` path — a second
  image for the same entity silently clobbers the file on disk, not just
  the DB row.
- **Cover Art Archive is under-used**: `CoverArtArchive.FetchFront`
  (`enrich/coverart.go:31-49`) only calls the `/release-group/{mbid}/front`
  convenience redirect. The real `/release-group/{mbid}` endpoint returns
  a full `images` array (each with `types` — Front/Back/Booklet/Medium/
  Tray/Obi/Spine/Track/Liner/Sticker/Poster/Watermark/Raw/Other — plus
  thumbnail and full URLs) and is never called.
- **Embedded art is single-picture everywhere**: `dhowden/tag` keeps only
  the last-parsed picture per file for every format it reads (ID3v2's
  `APIC` frames overwrite one map key; FLAC/Vorbis keep one `*Picture`
  field) — confirmed in the vendored source. `organize/copy.go`'s
  `readGenreAndArtwork` calls `m.Picture()` (singular) and
  `library.Store.InsertTrack`'s `saveEmbeddedAlbumArt` (`tracks.go:65-88`)
  only ever handles one image, gated by `SetAlbumArtIfOpen` so the first
  tagged track's picture wins and nothing after it can clobber that
  choice.
- **Frontend renders a single `<img>`** (`ArtistDetailPage.tsx`,
  `AlbumDetailPage.tsx`) with no gallery/carousel precedent anywhere in
  this codebase.

Two existing dependencies turn out to already support multi-picture
reading, which meaningfully de-risks part of this: `github.com/bogem/
id3v2` (already used for MP3 tag *writing*) exposes `Tag.GetFrames("APIC")`
— every attached-picture frame, not just one, each carrying its ID3v2
picture-type byte. `github.com/go-flac/go-flac` (already used for FLAC tag
writing) exposes every metadata block via `f.Meta`, and its sibling
package `github.com/go-flac/flacpicture` (same module family as the
already-used `flacvorbis`, added by this TDR) parses each `Picture`-typed
block into a `PictureType` + image bytes — the *same* numeric picture-type
table ID3v2 uses (0=Other, 3=FrontCover, 4=BackCover, 8=Artist, ...), so
one type→label mapping serves both formats. M4A (`covr` atom, repeated
`data` children) and OGG (repeated `METADATA_BLOCK_PICTURE` comments, each
itself a base64-encoded FLAC picture block — so the FLAC picture parser
above is reusable for OGG too, just base64-decoded first) have no existing
dependency and need hand-rolled container parsing, the same category of
work TDR 013 already did for WavPack's block format. WavPack itself just
needs a small extension to this project's own `apev2` package: APEv2
already supports multiple images via distinctly-keyed items ("Cover Art
(Front)", "Cover Art (Back)", "Cover Art (Booklet)", ...), it just isn't
collected today (`apev2.Read` only reads the "Front" key).

## 2. Alternatives Evaluated

### Alternative: schema — array/JSON columns vs. a real join table

- **A `photos`/`covers` array or JSON column on `artists`/`albums`** —
  Pros: no new table, no join. Cons: this app already made the opposite
  call for a structurally similar problem — `genres TEXT[]` is a flat list
  of strings with no per-entry metadata, but an image needs several
  fields per entry (thumb path, full path, source, picture type, primary
  flag, created-at for "first added" defaulting) — exactly the shape a
  relational row models naturally and an array/JSON column would just
  reimplement informally, losing FK-cascade-on-delete and straightforward
  per-row updates (`SET is_primary = true WHERE id = $1`) in the process.
- **New `artist_photos`/`album_covers` tables (chosen)** — one row per
  image, `FOREIGN KEY ... ON DELETE CASCADE` (an artist/album delete
  already needs to sweep its images the same way it sweeps everything
  else), a boolean `is_primary` column enforced single-true-per-entity at
  the application layer (matching how this codebase already enforces
  "empty name rows are permanently skipped" and similar invariants in Go
  rather than DB constraints). Cons: touches every List/Get query that
  reads `photo_thumb_path`/`cover_thumb_path` today (5 sites) to join in
  the primary row instead — a real but mechanical, one-time cost.

### Alternative: de-duplication — content hash vs. no de-duplication vs. perceptual hash

- **No de-duplication** — Pros: simplest. Cons: directly contradicts
  AC-5; a popular album's front cover is discoverable from up to three
  places (embedded tag, Cover Art Archive, a manual upload) and would
  otherwise show up to three times.
- **Perceptual/similarity hashing** (catches near-duplicates: same image
  re-compressed, resized, or watermarked differently) — Pros: catches
  more real-world duplicates than exact matching. Cons: a similarity
  threshold is a judgment call with false-positive risk (two genuinely
  different but similar-looking covers, e.g. a reissue's slightly
  different front, could get wrongly merged) — more machinery and more
  ways to surprise a user than this feature needs.
- **Exact content hash (chosen)** — SHA-256 over the raw image bytes
  before resizing, checked against every existing image for that entity
  before adding a new one. Pros: simple, deterministic, zero false
  positives. Cons: won't catch a re-encoded or re-sized version of the
  "same" cover — accepted; that's a source offering a genuinely different
  file, arguably still useful to keep rather than silently drop.

## 3. Structural Decision

### Schema (new migration `0006_multiple_artworks.sql`)

```sql
CREATE TABLE artist_photos (
    id BIGSERIAL PRIMARY KEY,
    artist_id BIGINT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    thumb_path TEXT NOT NULL,
    full_path TEXT NOT NULL,
    source TEXT NOT NULL,          -- 'upload' | 'embedded' | 'wikidata' | 'legacy'
    content_hash TEXT NOT NULL DEFAULT '',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_artist_photos_artist ON artist_photos (artist_id);

CREATE TABLE album_covers (
    id BIGSERIAL PRIMARY KEY,
    album_id BIGINT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    thumb_path TEXT NOT NULL,
    full_path TEXT NOT NULL,
    source TEXT NOT NULL,          -- 'upload' | 'embedded' | 'cover_art_archive' | 'legacy'
    picture_type TEXT NOT NULL DEFAULT '',  -- 'front'/'back'/'booklet'/... ; blank for plain uploads
    content_hash TEXT NOT NULL DEFAULT '',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_album_covers_album ON album_covers (album_id);

-- Preserve every existing single image as that entity's first, primary entry.
INSERT INTO artist_photos (artist_id, thumb_path, full_path, source, is_primary)
SELECT id, photo_thumb_path, photo_path, 'legacy', true FROM artists WHERE photo_path IS NOT NULL;
INSERT INTO album_covers (album_id, thumb_path, full_path, source, is_primary)
SELECT id, cover_thumb_path, cover_path, 'legacy', true FROM albums WHERE cover_path IS NOT NULL;

ALTER TABLE artists DROP COLUMN photo_thumb_path;
ALTER TABLE artists DROP COLUMN photo_path;
ALTER TABLE albums DROP COLUMN cover_thumb_path;
ALTER TABLE albums DROP COLUMN cover_path;
```

`art_status` stays exactly where it is — its meaning shifts subtly (from
"has *the* image been found" to "has automatic discovery been attempted
and settled for this entity") but its role in scheduling the background
job is unchanged, and manual upload/delete never touches it.

### Go/API shape

`ArtistPhoto`/`AlbumCover` structs (`ID`, `ThumbURL`, `FullURL`, `Source`,
`PictureType` (album only), `IsPrimary`, `CreatedAt`). `ArtistDetail`/
`AlbumDetail` gain `Photos []ArtistPhoto`/`Covers []AlbumCover` — populated
only on the single-entity fetch, matching how `Albums []Album` is already
detail-only, not carried on list rows. `Artist.PhotoThumbURL`/`PhotoURL`
(and the album equivalents) stay as the *primary* image's URLs, computed
via a `LEFT JOIN LATERAL ... WHERE is_primary = true LIMIT 1` in each of
the five existing queries that read them today (`ListArtists`, `GetArtist`
×2, `ListAlbums`, `GetAlbum`, `ListSongs`) — every list view and tile
keeps working unmodified against the same fields.

New `Store` methods (mirrored for artists/albums): `List*`, `Add*` (dedup
by content hash before inserting; auto-marks `is_primary` when it's the
entity's first image), `SetPrimary*`, `Delete*` (auto-promotes the
oldest remaining image to primary if the deleted one was primary).
`ImageStore.Save` gains a `hash` parameter, writing to
`<dir>/<kind>/<entityID>/<hash>/{thumb,full}.jpg` — content-addressed, so
the same bytes always land at the same path regardless of source.

### Cover Art Archive (Stage 2)

New `CoverArtArchive.FetchAll(ctx, releaseGroupMBID) ([]Image, error)`
against the real `/release-group/{mbid}` JSON endpoint, replacing
`FetchFront`. The background job downloads and adds every returned image
(each tagged with its CAA `types` as `picture_type`), instead of fetching
and setting just the front cover.

### Embedded multi-picture extraction (Stage 3)

A new shared "picture type" label table (0=Other, 3=FrontCover,
4=BackCover, 8=Artist, ... — the same numbers ID3v2 and FLAC both already
use) backs per-format extraction that bypasses `dhowden/tag` for this one
concern:
- **MP3**: `id3v2.Open` + `Tag.GetFrames("APIC")`, each cast to
  `id3v2.PictureFrame`.
- **FLAC**: iterate `go-flac`'s `f.Meta` for every `Picture`-typed block,
  parsed via the new `flacpicture` dependency.
- **OGG**: collect every repeated `METADATA_BLOCK_PICTURE` Vorbis comment,
  base64-decode, parse with the same FLAC picture-block parser.
- **M4A**: hand-rolled `covr` atom traversal collecting every `data` child
  (MP4 boxes have no per-picture type byte, so `picture_type` stays blank
  for this format).
- **WV**: extend `apev2.Read` to collect every `Cover Art (*)`-keyed item,
  not just `Cover Art (Front)`.

`readGenreAndArtwork` becomes "read genre + every embedded picture";
`InsertTrack`'s `saveEmbeddedAlbumArt` adds each one (subject to dedup)
instead of gating on a single first-tagged-track-wins write. "Retry
lookup" (TDR 007) keeps its existing reset-to-`pending`-and-rerun
mechanism, but the job it wakes now adds candidates rather than
overwriting.

## 4. Cross-Workspace Implications

- **`backend/`**: new migration `0006_multiple_artworks.sql`; new
  `ArtistPhoto`/`AlbumCover` types and Store methods (`library/`); `enrich/
  imagestore.go` (hash-addressed paths), `enrich/coverart.go` (`FetchAll`),
  `enrich/job.go` (adds instead of single-set); `organize/` (multi-picture
  extraction across MP3/FLAC/OGG/M4A, `apev2` extended for WV);
  `httpserver/catalog.go` (new list/add/set-primary/delete endpoints).
  New dependency: `github.com/go-flac/flacpicture/v2`.
- **`web/`**: new gallery component (mockup + sign-off before
  implementation, per this repo's process — this is new UI surface, not a
  minor tweak to an existing screen) on `ArtistDetailPage.tsx`/
  `AlbumDetailPage.tsx`, replacing the single `<img>`; upload/remove/
  set-primary actions per image.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: `artist_photos`/`album_covers` added; `artists.photo_thumb_path`/
  `photo_path` and `albums.cover_thumb_path`/`cover_path` removed, migrated
  into the new tables first (AC-9).
- Update `docs/ARCHITECTURE.md`'s artwork sections once each stage lands.
