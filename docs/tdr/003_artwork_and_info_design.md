# TDR 003: Artist & Album Artwork and Info

## 1. Context & Architectural Requirements

Per `docs/ARCHITECTURE.md` §6, "no artwork/cover-art pipeline" is explicitly
called out as deferred scope left over from [TDR 002](002_home_and_browsing_design.md).
Today `artists` and `albums` (added by TDR 002) carry only `name`/`title`,
`year`, and FK/count fields — no image, no descriptive metadata — and every
place that renders one (Albums/Artists index grids, Artist/Album detail
hero, Artist detail's album grid, Songs rows) falls back to a static SVG
circle-icon tile.

opusflow is currently 100% local-file-based: no streaming integration
(Spotify/Apple Music) or metadata API is wired up yet — those are described
in `docs/vision.md` as deliberate later work. This feature is the project's
first outbound network dependency of any kind, and the first background job
other than a directory scan.

The mockup (grilled and signed off before this doc) established the
product shape: real art fills the existing card/hero tiles; a refined
placeholder (not a generated gradient/initials) covers the "nothing found"
case; and both detail pages gain a facts-chip row plus an optional prose
bio/description sourced independently of the art.

## 2. Alternatives Evaluated

### Alternative: artwork sourcing — embedded tags only vs. external API only vs. both

- **Embedded tags only** — Pros: zero network dependency, fully offline,
  reuses the `dhowden/tag` dependency already in `scan`. Cons: tags only
  ever carry *album* art, never artist photos; any album whose files simply
  weren't tagged with art stays blank forever, with no way to fill the gap.
- **External API only** — Pros: one code path, works even for untagged
  libraries. Cons: throws away album art that's already sitting in the
  file, for free, with zero rate-limit/matching risk — strictly worse
  coverage and speed than checking locally first.
- **Both, embedded first (chosen)** — Pros: cheapest/most-reliable source
  tried first; external lookup only spent on the actual gap (albums with no
  embedded art, and all artists, since tags never carry artist photos).
  Cons: two extraction code paths to build and test instead of one.

### Alternative: external service — MusicBrainz + Cover Art Archive vs. Spotify Web API vs. Last.fm

- **Spotify Web API** — Pros: one API for both album art and artist photos,
  high coverage. Cons: requires registering an app and managing client
  credentials; `docs/vision.md` explicitly frames Spotify as a deliberate,
  later, user-facing integration (Web Playback SDK, opt-in per household) —
  pulling it in early just for images conflicts with that staging.
- **Last.fm API** — Pros: free API key, artist images available. Cons:
  smaller catalog than MusicBrainz, yet another vendor account to manage for
  a first pass, no meaningful advantage over MusicBrainz for this feature's
  needs.
- **MusicBrainz + Cover Art Archive (chosen)** — Pros: free, open, no
  API key/account (only a descriptive `User-Agent`, per their usage policy);
  matches `docs/vision.md`'s explicit "avoid proprietary protocols" stance
  and its already-planned future use of MusicBrainz for artist relationship
  data. Cons: strict ~1 req/sec rate limit (drives the background-job
  design below); artist photos require a second hop (MusicBrainz → Wikidata
  `P18` → Wikimedia Commons) since MusicBrainz artist entities carry no
  image themselves — lower hit rate than album art, accepted as best-effort
  per the signed-off mockup.

### Alternative: lookup timing — inline during scan vs. lazy on-demand vs. background job

- **Inline during scan** — Pros: simplest mental model, one pipeline. Cons:
  MusicBrainz's ~1 req/sec limit would make a multi-hundred-track import
  take drastically longer, and one slow/stuck lookup blocks the rest of the
  scan.
- **Lazy, on first page view** — Pros: no proactive fetching, scan stays
  untouched. Cons: the first view of any under-tagged item is slow, and an
  index page rendering 30 items at once would fan out a burst of requests
  that blow straight through the rate limit.
- **Background job, decoupled from scan (chosen)** — Pros: scan speed is
  unaffected (embedded-art extraction is local and cheap, so it stays
  inline in `scan`; only the external hop is deferred); a single paced
  worker respects the rate limit regardless of import size. Cons: art
  appears with a visible delay after a scan finishes, not instantly.
  Scoping the job to "every artist/album with `pending`/`failed` status",
  rather than to the directory just scanned, is what lets it double as the
  backfill mechanism for libraries that existed before this feature shipped
  — running it once at backend startup (in addition to after every scan)
  covers the case where an existing household upgrades without adding a
  new directory.

### Alternative: image storage — files on disk vs. Postgres bytea vs. hotlinked external URL

- **Postgres bytea** — Pros: one source of truth, no filesystem
  coordination. Cons: bloats the database with binary data Postgres isn't
  optimized to serve; every list-page load would round-trip image bytes
  through the app server instead of a cheap static-file response.
- **Hotlinked external URL** — Pros: no storage/caching code at all. Cons:
  breaks the self-hosted/offline promise (art vanishes if Cover Art
  Archive/Commons is down, or the household has no internet that moment);
  doesn't work at all for embedded-tag art, which has no URL to begin with.
- **Files on disk + DB path reference (chosen)** — Pros: keeps rows small,
  images survive restarts, served as a cheap static file response, works
  fully offline once cached. Cons: introduces a new persisted volume
  (`ARTWORK_DIR`) that needs to be backed up/mounted like `LIBRARY_ROOTS`,
  and a cleanup story if an artist/album is later deleted (out of scope
  here — orphaned files are harmless dead weight, not a correctness bug).

## 3. Structural Decision

**Matching**: one MusicBrainz search per artist/album (skipped entirely for
empty-name "Unknown Artist"/"Unknown Album" rows), accepting the top-ranked
result with no confidence threshold. The matched MusicBrainz ID is cached on
first success so later job runs (retrying a `failed` bio/facts lookup, say)
reuse it instead of re-searching by name.

**Independent statuses**: art, facts, and bio/description are tracked as
three separate `pending | found | not_found | failed` columns per
artist/album — the signed-off mockup's artist-detail state (facts found,
bio not found) depends on this. `not_found` is a terminal, accepted
negative; `failed` (network/rate-limit) is retried on the next job run.

**Background job**: a new `library/enrich` package (mirroring `library/scan`
as a sibling internal package) owns the MusicBrainz/Cover Art
Archive/Wikidata/Wikipedia HTTP clients (rate-limited to MusicBrainz's ~1
req/sec ceiling, descriptive `User-Agent` per their usage policy) and a
`Job` that `Service` runs as a background goroutine — the same pattern
`scan` already uses. Triggered from `main.go` once after migrations
complete, and from `Service` after each directory scan finishes. Each run
queries for artists/albums with any of the three statuses in
`pending`/`failed` and processes them one at a time at the rate-limited
pace.

**Images**: two variants generated at ingest time via `golang.org/x/image/draw`
(the maintained, dependency-light choice over a third-party imaging
library) — a grid-thumbnail size and a larger detail-hero size — written
under a new `ARTWORK_DIR` (mirrors `LIBRARY_ROOTS`/`STATIC_DIR`'s
env-var-configured-volume pattern) and served by a dedicated static file
route, distinct from the web app's `STATIC_DIR` route.

**Placeholder**: refined vinyl-disc glyph (album) / refined bust-silhouette
glyph (artist) on the existing ink-gradient tile — a deliberate redraw of
today's circle icon, not a generated gradient+initials treatment, per the
signed-off mockup.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - New migration: `artists` gains `photo_thumb_path`, `photo_path`,
    `art_status`, `formed_year`, `country`, `genres TEXT[]`, `facts_status`,
    `bio`, `bio_source_url`, `bio_status`, `musicbrainz_id`; `albums` gains
    the equivalent set (`cover_thumb_path`, `cover_path`, `art_status`,
    `label`, `country`, `genres TEXT[]`, `facts_status`, `description`,
    `description_source_url`, `description_status`, `musicbrainz_id`). All
    four status columns default `pending`.
  - New `backend/internal/library/enrich` package: MusicBrainz search +
    entity lookup client, Cover Art Archive client, Wikidata/Wikipedia
    client, image resize/store helpers, and the paced `Job` worker.
  - `library.Service` gains a method to run one enrichment pass, called from
    `main.go` (post-migration) and after `Service`'s existing scan-completion
    path.
  - New env var `ARTWORK_DIR` (like `LIBRARY_ROOTS`); new static route
    (indicative) `GET /artwork/{path}` serving it, parallel to the
    `STATIC_DIR` web-build route.
  - `Artist`/`AlbumDetail`/`Album`/`Song` JSON gain art URLs (thumb + full),
    facts (formed year/country/genres for artist, label/country/genres for
    album), and bio/description + source URL fields — exact field names
    decided at implementation time; a null/absent art URL is the client's
    signal to render the placeholder tile.
- **`web/`**:
  - `AlbumsPage`, `ArtistsPage`, `ArtistDetailPage`'s album grid: replace
    the circle-icon `.art`/`.avatar` div with an `<img>` (falling back to
    the refined placeholder tile when no URL is present).
  - `AlbumDetailPage`, `ArtistDetailPage`: hero art becomes an `<img>`; both
    gain a facts-chip row and an optional bio/description paragraph
    (rendered only when present, per AC-13), matching the signed-off
    mockup's new `.info-block`/`.fact-chip`/`.bio` styles.
  - `SongsPage`: each `.song-row` gains a small thumbnail matching the
    mockup's `.thumb`.
  - `web/src/api/library.ts`: extend `Artist`/`Album`/`AlbumDetail`/
    `ArtistDetail`/`Song` types with the new fields.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: third migration in the project — extends `artists`/`albums`
  only, no new tables (genres stored as a Postgres `TEXT[]` column rather
  than a join table, since they're read-only, MusicBrainz-sourced, and
  never queried/filtered by individually today).
- **Ops**: `docker-compose.yml` and the deploy compose file gain a new
  `ARTWORK_DIR` volume mount, alongside the existing library-root mounts.
- Update `docs/ARCHITECTURE.md` §3 (new `library/enrich` component, new
  static route), §4 (extended `artists`/`albums` columns), §5 (decision-log
  row linking here), and §6 (remove the "no artwork/cover-art pipeline"
  deferred-work bullet this feature closes out) once implementation lands.
