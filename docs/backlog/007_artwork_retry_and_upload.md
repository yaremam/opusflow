# User Story: Artwork Status, Manual Retry & Upload

## 1. User Value Statement

As a **household member browsing the library**, I want to **see when an
artist photo or album cover couldn't be found automatically, and be able to
either retry the lookup or upload my own image**, So that **a stubborn
placeholder tile isn't a dead end — I can always get real art on an artist
or album, one way or another**.

## 2. Strict Acceptance Criteria

Scope: **Art status only** (artist photo / album cover). Facts and
Bio/Description ([TDR 003](../tdr/003_artwork_and_info_design.md)) are
unchanged — still silent, best-effort, no notification/retry/upload for
them.

### Status surfacing

- **AC-1**: The JSON API for an artist/album now includes its Art status
  (`pending` / `found` / `not_found` / `failed`) alongside the existing
  derived `photoUrl`/`photoThumbUrl` (or `coverUrl`/`coverThumbUrl`) fields —
  today the API collapses all four states into an empty string, which is no
  longer enough to distinguish "still looking" from "gave up."
- **AC-2**: Everywhere an artist/album's art is rendered without a real
  image (Albums grid, Artists grid, Artist detail's album grid, Songs list
  row thumbnail), a small corner badge appears on the placeholder tile when
  status is `not_found` (amber) or `failed` (red) **and there's no image to
  show**. A `pending` item (never looked at yet) shows no badge — same
  placeholder as today.
- **AC-3**: The Artist detail and Album detail pages show a status pill next
  to the art (amber "No artwork found" for `not_found`, red "Artwork lookup
  failed" for `failed`) under the same "and there's no image to show" rule
  as AC-2 — no pill when `found`, `pending`, or when a status of
  `not_found`/`failed` still has a previously-found image sitting alongside
  it (AC-11 covers how that combination can happen).

### Manual retry

- **AC-4**: Both the Artist detail and Album detail pages show a "Retry
  lookup" action for their art, regardless of current status — including
  when art was already `found`, in case the automatic match was wrong.
- **AC-5**: Clicking retry resets that artist/album's Art status to
  `pending` and immediately wakes the background enrichment job (rather than
  waiting for the next import or the next backend restart) — the request
  itself returns right away, it does not block on the lookup completing.
- **AC-6**: After clicking retry, the page polls that artist/album for up to
  ~30 seconds and updates in place once its status leaves `pending` (new art
  appears, or the status pill switches to reflect the new outcome); polling
  simply stops after the window if it's still pending, with no error shown
  for that.

### Manual upload

- **AC-7**: Both the Artist detail and Album detail pages show an "Upload
  photo"/"Upload cover" action for their art, regardless of current status —
  same always-available treatment as retry.
- **AC-8**: Clicking upload opens the OS's native file picker
  (`accept="image/*"`) — no in-app dropzone/modal. JPEG, PNG, and GIF are
  accepted (whatever formats the existing image decoder already supports);
  anything else is rejected with a clear error. A single upload is capped at
  8 MB.
- **AC-9**: An uploaded image is saved synchronously (no queueing/polling):
  the same thumbnail + full-size variants the automatic lookup already
  produces are generated immediately, the artist/album's Art status becomes
  `found`, and the page reflects the new image as soon as the upload
  response comes back.
- **AC-10**: A manually-uploaded image is never overwritten by a later
  automatic background run — the enrichment job only ever processes
  `pending`/`failed` items, and upload always leaves the item `found`.
- **AC-11**: Retrying an already-`found` artist/album (AC-4) never destroys
  its existing image while the retry is in flight or if the retry comes up
  `not_found`/`failed` this time — a previously-found image is only ever
  replaced by a new one, never cleared just because a later lookup attempt
  didn't find (or failed to find) a replacement.
