# TDR 017: Dedup Artists/Albums by MusicBrainz ID

## 1. Context & Architectural Requirements

GitHub issue #30, "Duplicate artists from tag inconsistencies (e.g.
Cyrillic/Latin homoglyphs) — dedup by MusicBrainz ID instead of exact name
match" (bug + enhancement, no further written spec).

Confirmed by reading the code: `upsertArtist`/`upsertAlbum`
(`backend/internal/library/catalog.go:211-239`) match purely on
`ON CONFLICT (name)` / `ON CONFLICT (title, artist_id)` — a byte-exact
Postgres unique constraint (`0002_artists_albums.sql:5,14`), no
normalization, case-folding, or transliteration anywhere. Two tags that
are visually the same artist but not byte-identical ("Пётр" vs "Petr" —
different alphabets entirely, not just a Unicode-normalization gap; or
just stray whitespace/casing) create two separate `artists` rows today.

A MusicBrainz ID column already exists and is already being populated:
`artists.musicbrainz_id`/`albums.musicbrainz_id`
(`0003_artwork_and_info.sql:3,18`), written by
`Store.SetArtistMusicBrainzID`/`SetAlbumMusicBrainzID`
(`enrich_store.go:81-87,157-163`) from inside the background enrichment
job's `processArtist`/`processAlbum` (`enrich/job.go:146-160,260-270` —
TDR 003), which runs unconditionally for every artist/album still owed
art/facts/bio, resolving a MusicBrainz match by name search. So the data
needed to detect a duplicate — two rows independently resolving to the
same MBID — is already collected today; nothing reads it back to notice
the collision.

Synchronous MusicBrainz matching *before* a row is created (matching by
MBID from the start, never creating the duplicate in the first place) was
considered and rejected: TDR 003 deliberately made enrichment
asynchronous specifically because MusicBrainz's ~1 req/sec rate limit
would make import itself slow, and would make import require network
availability, both of which the current design explicitly avoids
(`docs/tdr/003_artwork_and_info_design.md`). This TDR keeps import
exactly as fast/offline-capable as it is today, and instead detects and
resolves the duplicate once enrichment (which already runs async, already
network-dependent) catches it.

## 2. Alternatives Evaluated

### Alternative: how to prevent/catch duplicates

- **Name normalization at upsert time** (Unicode NFC, trim, case-fold) —
  Pros: cheap, no network call, catches whitespace/casing inconsistencies
  immediately at import. Cons: doesn't touch the issue's actual headline
  example — "Пётр" vs "Petr" are genuinely different alphabets, not a
  normalization-form difference, so this wouldn't merge them. Would need
  a maintained transliteration/confusables table to go further, which is
  its own ongoing-accuracy liability (false-positive merges of two
  different artists that happen to transliterate the same way).
- **Synchronous MusicBrainz match at import** — Pros: would prevent the
  duplicate row from ever being created. Cons: re-introduces the exact
  rate-limit/offline problem TDR 003 designed around; rejected (see above).
- **Async merge-on-shared-MBID once enrichment resolves it (chosen)** —
  Pros: reuses data already being collected, no new network calls beyond
  what enrichment already makes, keeps import synchronous-and-offline as
  today. Cons: a duplicate is still visible for the window between import
  and the next enrichment run (typically seconds to minutes, per TDR 003's
  post-scan trigger) — accepted, since AC-1 only promises resolution once
  MusicBrainz confirms the match, not instantaneously at import.

### Alternative: which row survives a merge

- **Keep whichever row MusicBrainz matched most recently** — Cons:
  non-deterministic from the user's perspective (depends on job scheduling
  order), and risks flip-flopping which row's incidental state (e.g. which
  photo is primary) "wins" across repeated runs.
- **Keep the lower ID / earlier-created row (chosen)** — deterministic,
  independent of processing order, and matches the intuitive "the one that
  was here first stays" behavior this app already uses elsewhere (e.g.
  `deleteGalleryRow`'s primary-promotion picks the oldest remaining image,
  `artwork_gallery.go:203-212`).

## 3. Structural Decision

### New Store primitives (`backend/internal/library/merge.go`)

```go
func (s *Store) FindArtistIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error)
func (s *Store) MergeArtists(ctx context.Context, loserID, winnerID int64) error

func (s *Store) FindAlbumIDByMusicBrainzID(ctx context.Context, mbid string, excludeID int64) (int64, bool, error)
func (s *Store) MergeAlbums(ctx context.Context, loserID, winnerID int64) error
```

`MergeArtists`, in one transaction:
1. For each of the loser's albums: if the winner already has an album
   with the same title, fold the loser album's tracks and covers into
   that existing album (shared `mergeAlbumRows` helper — also `MergeAlbums`'s
   own implementation, see below) instead of moving the album row itself
   (AC-3); otherwise just reassign the album row's `artist_id`.
2. Reassign every remaining `tracks.artist_id` and `artist_photos.artist_id`
   from loser to winner.
3. `dedupeSingleFlag` (a small shared helper, parameterized by
   table/fk-column/flag-column) collapses `is_primary` back down to at
   most one `true` row on the merged gallery (AC-4): keep whichever was
   already flagged (oldest on a tie), clear the rest. Written generically
   enough that covering TDR 016's separate `is_banner` flag too, once that
   ships, is a one-line addition at each call site — not on `main` as of
   this entry, so out of scope for now.
4. `DELETE FROM artists WHERE id = loser` — safe now that every child row
   has already been moved off it.

`MergeAlbums` is `mergeAlbumRows` (reassign `tracks.album_id`/
`album_covers.album_id`, dedupe the primary cover, delete the loser
album) wrapped in its own transaction — used both directly (issue #31's manual
album merge) and internally by `MergeArtists` for AC-3's same-titled-album
case. Guarded to only merge albums under the same artist — a
different-artist merge is out of scope here (that's an artist merge, not
an album one).

No file movement (AC-6): every `tracks.path`/`artist_photos.thumb_path`
etc. stays exactly as organize-on-import or `ImageStore` wrote it. The
row's catalog association changes; the bytes on disk don't move.

### Wiring into the enrichment job

`enrich.Store` (`target.go:48-62`) gains the four methods above.
`processArtist`/`processAlbum` (`enrich/job.go`), right after `mbid` is
determined — whether freshly resolved via `SearchArtist`/
`SearchReleaseGroup` or already cached from a prior run, covering both
paths per AC-1 — call `FindArtistIDByMusicBrainzID`/
`FindAlbumIDByMusicBrainzID`. On a hit, compute winner/loser by ID (AC-2)
and call `MergeArtists`/`MergeAlbums`; a merge error is logged and that
item's `RunSummary` stays empty rather than aborting the run (AC-7,
matching every other per-item failure path in this job). If the
currently-processing row (`a`/`al`) turns out to be the loser, `process*`
returns immediately — the row it was about to keep enriching no longer
exists. If it's the winner, processing continues normally (now possibly
carrying tracks/photos just merged in from the loser).

## 4. Cross-Workspace Implications

- **`backend/`**: new `library/merge.go` (`MergeArtists`/`MergeAlbums`
  plus the two `FindByMusicBrainzID` lookups and the shared
  `dedupeSingleFlag`/`mergeAlbumRows` helpers); `enrich/target.go` (`Store`
  interface gains the four methods); `enrich/job.go` (`processArtist`/
  `processAlbum` call the merge check). No schema change — both
  `musicbrainz_id` columns already exist. No new dependencies.
- **`web/`**: none — this is a background-job behavior change with no UI
  surface of its own.
- **`mobile/`**: out of scope, unchanged.
- Retroactive dedup of artists/albums that duplicated *before* this ships
  is explicitly not this entry's job — see backlog 018 (issue #31)'s
  manual merge tool, which reuses `MergeArtists`/`MergeAlbums` directly.
