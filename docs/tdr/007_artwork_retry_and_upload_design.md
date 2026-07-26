# TDR 007: Artwork Status, Manual Retry & Upload

## 1. Context & Architectural Requirements

[TDR 003](003_artwork_and_info_design.md) built the background enrichment
job and its independent `pending | found | not_found | failed` status per
kind (art, facts, bio/description), but the JSON API only ever exposes the
*derived* image URL — an empty string covers "still pending," "confirmed
nothing exists," and "the lookup errored" identically, and the client can't
tell them apart. There's also no way to nudge a stuck item: the job only
runs after a new import completes or once at backend startup (TDR 003 §3),
so a `failed` item (a transient MusicBrainz/network error) can sit
unprocessed indefinitely if no further import ever happens, and a
`not_found` item is never retried automatically at all, by design.

This feature (grilled and mocked up, signed off before this doc) surfaces
that gap and gives the household two escape hatches, scoped to **art only**
— Facts and Bio/Description stay exactly as TDR 003 left them, silent and
best-effort:

- **Notify**: a status badge on every grid tile that renders art, plus a
  status pill on the two detail pages, whenever `not_found`/`failed` has no
  image to fall back on.
- **Retry**: a manual, always-available nudge (even on an already-`found`
  item) that re-runs just that one artist/album's art lookup.
- **Upload**: a manual, always-available image upload that bypasses
  MusicBrainz/Cover Art Archive entirely.

Digging into `SetArtistArt`/`SetAlbumArt` (`backend/internal/library/enrich_store.go`)
while designing retry surfaced a real correctness risk this TDR has to
close: today, any write for a `not_found`/`failed` outcome passes empty
thumb/full paths straight through to the `UPDATE`, nulling out whatever was
there — harmless today only because nothing ever routes an already-`found`
item back through the job. Making retry "always available" is the first
thing that would exercise that path, so §3 fixes the write semantics rather
than special-casing retry around the bug.

## 2. Alternatives Evaluated

### Alternative: exposing status to the API — raw status enum vs. a derived boolean

- **Derived boolean** (e.g. `artFailed: true`) — Pros: smaller API surface,
  one flag to check. Cons: collapses `not_found` and `failed` into the same
  signal, but the mockup deliberately distinguishes them (amber "nothing
  matched" vs. red "an error happened, worth retrying") — a boolean can't
  drive that.
- **Raw status enum (chosen)** — Pros: the frontend already needs
  `pending`/`found`/`not_found`/`failed` as four distinct render states
  (badge color, pill copy, whether retry/upload even need special
  treatment); shipping the same enum the backend already tracks needs no
  translation layer. Cons: couples the wire format to the internal status
  vocabulary, so renaming a status later touches the API too — accepted,
  since the vocabulary is already stable since TDR 003.

### Alternative: manual retry — synchronous single-item lookup vs. queue + wake the background job

- **Synchronous** — Pros: no polling, the button's own response carries the
  final result. Cons: a single lookup is still 1-2 MusicBrainz/Cover Art
  Archive round trips inline in an HTTP request; ties up a request goroutine
  for something that already has an established async pattern in this
  codebase.
- **Queue + wake the job (chosen)** — Pros: reuses `Job.Run` completely
  unchanged — retry only has to reset one row's `art_status` to `pending`
  and launch `enricher.Run(context.Background())` in a goroutine, the exact
  pattern `ConfirmImport` already uses after a copy finishes
  (`service.go`); no new lookup code path to build or rate-limit
  separately. Cons: needs the frontend to poll rather than trust the
  response body — solved with a short-lived (~30s) poll loop, same shape as
  any "processing" state this app already renders (e.g. an in-progress
  import).

### Alternative: preserving a previously-found image across a failed retry — special-case retry vs. fix the setter's write semantics

- **Special-case in retry** — Pros: touches only the new retry code path.
  Cons: the underlying bug (a `not_found`/`failed` write nulling existing
  paths) is still live for any other future caller of `SetArtistArt`/
  `SetAlbumArt`; papering over it in one call site is a bandaid, not a fix.
- **Fix `SetArtistArt`/`SetAlbumArt` directly (chosen)**: a write only
  touches the path columns when the incoming status is `Found`; a
  `not_found`/`failed` write updates only the status column, leaving
  whatever path was already there untouched. Pros: correct for every
  caller, present and future, not just retry; matches AC-11's intent
  precisely — "a previously-found image is only ever replaced by a new one,
  never cleared." Cons: none identified — this is strictly more correct
  than today's behavior, and the two call sites (art resolution's `Found`
  and `NotFound`/`Failed` branches) already always pass empty strings for
  the latter, so no caller relies on the old clearing behavior.

### Alternative: manual upload — reuse `ImageStore.Save` vs. a parallel upload-specific pipeline

- **Parallel pipeline** — Pros: none identified. Cons: `ImageStore.Save`
  (`enrich/imagestore.go`) already takes arbitrary bytes + a `kind`/`id` and
  produces the same thumb/full variants the automatic lookup uses —
  duplicating that would be pure, unjustified extra code.
- **Reuse `ImageStore.Save` (chosen)**: the upload handler decodes the
  multipart body, validates size/format, and calls the same `Save` the job
  already calls, then writes `Found` directly via `SetArtistArt`/
  `SetAlbumArt` (no `Job`/MusicBrainz involvement at all — upload is a
  complete bypass of the lookup, as scoped). Pros: one image-writing code
  path total; upload and automatic lookup produce byte-for-byte identical
  storage layout. Cons: none.

## 3. Structural Decision

**API**: `Artist`/`Album` (and their `*Detail` variants) gain an
`artStatus` field (`"pending" | "found" | "not_found" | "failed"`)
alongside the existing `photoUrl`/`photoThumbUrl` (`coverUrl`/
`coverThumbUrl`) fields — no new endpoint needed for status alone, since
every place that fetches an artist/album already gets this struct.

**Retry**: new endpoints `POST /api/library/artists/{id}/art/retry` and
`POST /api/library/albums/{id}/art/retry`. Each: resets that row's
`art_status` to `pending` via a new `Store.ResetArtistArt`/
`ResetAlbumArt` (status only — leaves path columns untouched, so a
previously-found image keeps rendering while the retry is in flight),
then launches `enricher.Run(context.Background())` in a goroutine exactly
like `ConfirmImport` does, and returns `202 Accepted` immediately. The
frontend polls `GET /api/library/artists/{id}` (or the album equivalent)
every ~2s for up to ~30s, stopping once `artStatus` is no longer `pending`
(or the window elapses).

**Setter fix**: `SetArtistArt`/`SetAlbumArt`'s `UPDATE` only writes the
path columns when `status == Found`; a `NotFound`/`Failed` write touches
only `art_status`. This makes "retry on an already-found item" safe by
construction, and is what makes AC-11 hold without any special-casing in
the retry handler itself.

**Effective badge/pill logic** (AC-2/AC-3): rendered whenever
`artStatus` is `not_found`/`failed` **and** the corresponding URL field is
empty — never based on `artStatus` alone. This is the direct consequence of
the setter fix above: a `not_found`/`failed` status can now coexist with a
perfectly good, previously-found image (a retry that didn't pan out this
time), and that combination must render as "has art," not as a failure
banner sitting next to a real photo.

**Upload**: new endpoints `POST /api/library/artists/{id}/art` and
`POST /api/library/albums/{id}/art` (multipart, one `image` field, 8MB
cap). The handler decodes the upload with the same `image.Decode` already
imported by `ImageStore` (JPEG/PNG/GIF, whatever's registered — no new
format support added), rejects anything else with `400`, calls
`ImageStore.Save(kind, id, data)`, then `SetArtistArt`/`SetAlbumArt(ctx,
id, Found, thumbURL, fullURL)` directly — no `Job`, no MusicBrainz, no
queueing. Responds `200` with the updated artist/album so the frontend
updates in place with no polling.

**Frontend**: `ArtTile` gains an optional `badge` indicator (rendered only
under the effective-badge rule above), used by every grid/list context that
already renders one. `ArtistDetailPage`/`AlbumDetailPage` gain a status
pill (same rule) plus a "Retry lookup" and "Upload photo/cover" action pair
next to the art — bordered buttons when there's no image to show (AC-2's
badge case), quieter ghost-style buttons when art is already present, per
the signed-off mockup. Upload uses a plain `<input type="file"
accept="image/*">` triggered by the button — no dropzone/modal, since it's
a single file, not a folder.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - `backend/internal/library/enrich_store.go`: fix `SetArtistArt`/
    `SetAlbumArt` to only write path columns on a `Found` status (§3); add
    `ResetArtistArt`/`ResetAlbumArt` (status-only reset to `pending`).
  - `backend/internal/library/catalog.go`: `Artist`/`Album`/`ArtistDetail`/
    `AlbumDetail` structs gain `artStatus` (`json:"artStatus"`).
  - `backend/internal/library/service.go`: `Service` gains
    `RetryArtistArt(id)`/`RetryAlbumArt(id)` (reset + launch
    `enricher.Run` in a goroutine, mirroring `ConfirmImport`'s existing
    pattern) and `UploadArtistArt(id, data)`/`UploadAlbumArt(id, data)`
    (decode/validate/`ImageStore.Save`/`SetArt...(Found, ...)`).
  - `backend/internal/httpserver`: four new routes — `POST
    /api/library/artists/{id}/art/retry`, `POST
    /api/library/albums/{id}/art/retry`, `POST
    /api/library/artists/{id}/art`, `POST /api/library/albums/{id}/art`.
  - No schema migration — `art_status`/path columns already exist (TDR
    003); this only changes how they're written and adds no columns.
- **`web/`**:
  - `web/src/api/library.ts`: `Artist`/`Album` types gain `artStatus`; new
    `retryArtistArt(id)`/`retryAlbumArt(id)` and
    `uploadArtistArt(id, file)`/`uploadAlbumArt(id, file)` calls (the
    latter two POST a single-field `FormData`, no progress tracking needed
    for one small image — plain `fetch`, not the import flow's `XMLHttpRequest`
    pattern).
  - `web/src/components/ArtTile.tsx`: optional badge overlay, shown per the
    effective-badge rule.
  - `web/src/pages/ArtistDetailPage.tsx`, `AlbumDetailPage.tsx`: status
    pill + retry/upload action pair, short-lived poll loop after retry.
  - `web/src/pages/ArtistsPage.tsx`, `AlbumsPage.tsx`,
    `web/src/pages/HomePage.tsx` (recently-added rows), and the Artist
    detail page's own album grid: no code change beyond passing the new
    `artStatus` field through to `ArtTile` — the grid/list pages already
    render `ArtTile` generically.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none — see above.
- Update `docs/ARCHITECTURE.md` §3 (new retry/upload endpoints,
  `enrich_store.go`'s corrected write semantics), and §6 (this closes the
  "no manual override" gap TDR 003 left implicit) once implementation
  lands.
