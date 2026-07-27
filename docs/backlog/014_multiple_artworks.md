# User Story: Multiple Artworks per Artist/Album

## 1. User Value Statement

As a **household member browsing artists and albums**, I want to **see and
manage more than one image per artist/album — front cover, back cover,
booklet scans, several artist photos — instead of being stuck with
whichever single image was found or uploaded last**, So that **the
library reflects the artwork I actually have, not just one arbitrary
picture per entity**.

## 2. Strict Acceptance Criteria

- **AC-1**: An artist/album detail page shows every image currently
  associated with it as a gallery, not just one.
- **AC-2**: Exactly one image per artist/album is marked primary — that's
  the one shown in list views and grid tiles (where only one fits).
  Defaults to whichever image was found/added first; the reviewer can
  change it at any time via a "Set as primary" action on any other image
  in the gallery.
- **AC-3**: Uploading an image always adds a new gallery entry — it never
  replaces an existing one.
- **AC-4**: Every image in the gallery has its own "Remove" action (reusing
  this app's existing keep-vs-delete-file-on-disk confirmation pattern),
  independent of the others.
- **AC-5**: Before a newly-found or newly-uploaded image is added, its
  content is compared (by hash) against every image already in that
  entity's gallery — an exact duplicate is skipped rather than added
  again.
- **AC-6**: The background enrichment job fetches every image type Cover
  Art Archive has for an album's matched release (Front, Back, Booklet,
  Medium, Tray, Obi, Spine, Track, Liner, Sticker, Poster, Watermark,
  Raw/Other) — not just the front cover, as today.
- **AC-7**: Every picture embedded in a track's own tags is extracted, not
  just one — across every supported audio format (MP3, FLAC, M4A, OGG,
  WavPack).
- **AC-8**: Re-running automatic matching ("Retry lookup") adds any newly
  found images (subject to AC-5's dedup) rather than replacing the
  existing gallery.
- **AC-9**: Existing artists'/albums' current single image is preserved
  across the upgrade — it becomes that entity's first gallery entry,
  marked primary, rather than being lost.

## 3. Delivery Staged Across Three PRs

Given the size, this ships in three independently-mergeable stages (see
TDR 014 §3 for the full design each draws from):

1. **Foundation** — new gallery schema, Store/Service methods, manual
   multi-upload with primary-selection and per-image removal, and the
   frontend gallery UI. Delivers AC-1 through AC-5, AC-9 on its own.
2. **Cover Art Archive** — full typed image set instead of just the front
   cover (AC-6), feeding the same gallery the foundation stage built.
3. **Embedded multi-picture extraction** — every picture in a track's own
   tags, across all five supported formats (AC-7), plus "Retry lookup"
   adding rather than replacing (AC-8).
