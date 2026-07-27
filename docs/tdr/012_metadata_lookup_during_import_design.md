# TDR 012: Metadata Lookup During Import

## 1. Context & Architectural Requirements

GitHub issue #17 asked for a way to manually look up an artist/album/track
listing while reviewing an import, for files whose tags are missing or
wrong. Research before grilling this found the existing enrichment system
(`backend/internal/library/enrich`, TDR 003) doesn't cover this at all:

- `MusicBrainz.SearchArtist`/`SearchReleaseGroup`
  (`enrich/musicbrainz.go:74-105`) return only the **top-ranked match**
  (`resp.Artists[0].ID`) — no list of candidates a person could choose
  from, because the only caller today (`enrich.Job`, run against catalog
  rows already in the database) doesn't need one; TDR 003 accepted "best
  match, no confidence threshold" for a silent background job. An
  interactive picker needs the opposite: every plausible candidate, with
  enough detail (disambiguation, year, track count) for a person to choose
  correctly.
- MusicBrainz's **release-group** (what `LookupReleaseGroup` fetches, and
  what opusflow's "album" already maps to) carries no track listing at
  all — that lives on a **release** (a specific pressing/edition:
  a particular country's CD, a remaster, a reissue with bonus tracks,
  etc.), a finer-grained entity nothing in this codebase queries today.
  Getting real track numbers/titles means a new integration against
  MusicBrainz's `/release/{mbid}?inc=recordings` endpoint.
- The MusicBrainz client is rate-limited to 1 request/second via a shared
  mutex-based limiter (`enrich/ratelimit.go`) living on the one
  `*MusicBrainz` instance the app constructs. That instance is currently
  only ever created when `ARTWORK_DIR` is set
  (`cmd/server/main.go:67-79`) — bundled in with the artwork-specific
  `ImageStore`/Cover Art Archive/Wikidata wiring, since today's only
  consumer (the background enrichment job) needs all of those together.
  This lookup feature needs none of that (no image storage, no bio/genre
  fetching) — just the same rate-limited MusicBrainz HTTP client.

## 2. Alternatives Evaluated

### Alternative: search flow shape — sequential artist→album vs. one combined search

- **Combined search** (reusing `SearchReleaseGroup(title, artist)`
  roughly as-is, both typed into one box) — Pros: fewest clicks, closest
  to existing code. Cons: MusicBrainz's relevance ranking over a combined
  free-text query mixes results across any artist whose name loosely
  matches, with no way to first pin down "no, the *other* band called
  that" before seeing album results — confusing exactly when it matters
  most (obscure/ambiguous artist names, the case this feature exists for).
- **Sequential: pick artist, then browse only their albums (chosen)** —
  Pros: matches the issue's own description ("pulls artist metadata and
  can assign album..."); once an artist's MBID is confirmed, browsing
  their release-groups (`GET /release-group?artist={mbid}`, MusicBrainz's
  browse-by-relation mode rather than a text search) can't return a
  wrong-artist result at all. Cons: one more step/round-trip than a
  combined search — accepted, since each step is short (pick from a
  list) and the rate limit means any flow already involves a brief wait
  between searches regardless.

### Alternative: track matching — by row order vs. manual per-file assignment

- **Manual per-file assignment** (a dropdown per file, pick its matching
  release track) — Pros: zero risk of a wrong pairing when file order
  doesn't match the release's track order (e.g. files named/sorted
  oddly). Cons: for a full album that's as many manual decisions as just
  typing the numbers in by hand — defeats the point of pulling a track
  listing at all.
- **Match by current row order (chosen)** — zip the release's tracks
  (sorted by position) against the album's files in whatever order they
  already appear in the review table. Pros: one click fills in an entire
  album correctly in the overwhelmingly common case (files already sorted
  by track number or filename); AC-5's "leftover" handling means a count
  mismatch degrades gracefully — extra files/tracks are simply left
  unmatched, not silently mis-paired — and any wrong pairing is still just
  as fixable afterward as a bad tag is today, via the same per-track
  Title/Track# inputs that already exist. Cons: a genuinely out-of-order
  file list could get mismatched silently — accepted, since the existing
  manual fields are the fallback either way and this is the same
  trust-the-common-case trade-off TDR 011's "match excluded tracks by
  index" already made.

### Alternative: MusicBrainz client wiring — reuse the artwork-gated job's client vs. an independently-constructed one

- **Reuse `enrich.Job`'s client** (only available when `ARTWORK_DIR` is
  set) — Pros: one fewer client instance/no wiring change. Cons: would
  silently disable this entire feature for any `go run` without
  `ARTWORK_DIR` set (the normal local-dev path, per `cmd/server/main.go`'s
  own comments) even though nothing about a text search needs the image
  storage that flag actually gates — an arbitrary coupling this feature
  has no reason to inherit.
- **Construct a `*enrich.MusicBrainz` unconditionally in `main.go`, wired
  into `library.Service` via a new setter (chosen)** — Pros: matches the
  existing `SetEnricher`/`SetImages` setter pattern exactly (`service.go`
  already does this for optional dependencies); the client itself has no
  filesystem dependency (just an HTTP client + rate limiter + User-Agent),
  so nothing is lost by constructing it always. Cons: one more
  long-lived client instance when `ARTWORK_DIR` is unset — negligible, it
  holds no resources beyond a mutex and an `http.Client`.

## 3. Structural Decision

**Backend** — `enrich/musicbrainz.go` gains list-returning methods
alongside (not replacing) the existing top-match-only ones the background
job uses:
- `SearchArtists(ctx, name) ([]ArtistMatch, error)` — `ArtistMatch{MBID,
  Name, Disambiguation}` for every result, not just the first.
- `ArtistReleaseGroups(ctx, artistMBID) ([]ReleaseGroupMatch, error)` — a
  *browse* call (`?artist={mbid}`, not `?query=`), returning
  `{MBID, Title, FirstReleaseYear}`.
- `ReleaseGroupReleases(ctx, releaseGroupMBID) ([]ReleaseMatch, error)` —
  browse call (`?release-group={mbid}&inc=media`), returning `{MBID,
  Country, Date, TrackCount}` per release so the picker can show AC-4's
  disambiguating detail.
- `ReleaseTracks(ctx, releaseMBID) ([]Track, error)` — the new
  `/release/{mbid}?inc=recordings` integration, returning `{Position,
  Title}` per track, flattening a release's `media[].tracks[]`.

`library.Service` gains a `musicBrainz *enrich.MusicBrainz` field and
`SetMusicBrainzSearch(*enrich.MusicBrainz)` setter (same shape as
`SetEnricher`/`SetImages`), plus thin passthrough methods for the four
calls above. `cmd/server/main.go` constructs `enrich.NewMusicBrainz(...)`
unconditionally (§2) and calls the new setter, independent of the
`artworkDir != ""` block.

New endpoints in `httpserver` (all `GET`, thin handlers over the `Service`
passthroughs, same shape as existing `handleList*` handlers):
`/api/metadata/artists?q=`, `/api/metadata/artists/{mbid}/release-groups`,
`/api/metadata/release-groups/{mbid}/releases`,
`/api/metadata/releases/{mbid}/tracks`.

**Frontend** — a new `MetadataLookupModal` component (using the existing
`.modal-scrim`/`.modal-panel` pattern from `RemoveModal.tsx`, the closest
existing modal precedent, extended with a step indicator for the four
stages: artist → album → release → track review). `ImportPage.tsx` gets a
"Look up metadata" button per album group (next to the existing
`Overwrite all`/tri-state-checkbox controls) that opens the modal scoped
to that `albumIndex`. The modal owns its own search-in-progress state
entirely internally; on its final Apply it calls back into `ImportPage`
with the resolved `{artist, album, year, tracks: {position, title}[]}`,
which `ImportPage` folds into the plan (mirroring `withAlbumField`/
`withTrackField`, matching AC-6/AC-7's "everything applies as one commit,
then revalidate" behavior already established for `withOverwriteAlbum`).

## 4. Cross-Workspace Implications

- **`backend/`**: `enrich/musicbrainz.go` (new list/browse/track-listing
  methods), `library/service.go` (new field + setter + passthrough
  methods), `httpserver/` (four new `GET` endpoints + handler functions),
  `cmd/server/main.go` (unconditional `MusicBrainz` construction, wired via
  the new setter).
- **`web/`**: new `web/src/components/MetadataLookupModal.tsx` +
  `.css`; `web/src/pages/ImportPage.tsx` gains the per-album trigger button
  and the apply-callback wiring; `web/src/api/library.ts` gains client
  functions for the four new endpoints.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none — this only ever writes into the same Plan fields
  (Artist/Album/Year/Title/TrackNumber) manual editing already writes into;
  nothing new is persisted until the existing confirm/copy path runs.
- Update `docs/ARCHITECTURE.md`'s enrichment section once implemented: the
  MusicBrainz client is no longer constructed only when `ARTWORK_DIR` is
  set, and it now has two consumers (the background `enrich.Job` and this
  on-demand search) sharing one rate limiter.
