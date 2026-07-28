# User Story: Dedup Artists/Albums by MusicBrainz ID

## 1. User Value Statement

As a **household member importing music tagged by different tools over
time**, I want **artists and albums that MusicBrainz confirms are the same
entity to be recognized as one row, even when their tags don't match
exactly (different capitalization, stray whitespace, a Cyrillic/Latin
homoglyph)**, So that **my library doesn't accumulate visually-duplicate
artists/albums that split one artist's tracks and albums across multiple
rows**.

## 2. Strict Acceptance Criteria

- **AC-1**: When the background enrichment job resolves an artist's
  MusicBrainz ID — whether freshly searched or already cached from an
  earlier run — and another existing artist row already carries that same
  MusicBrainz ID, the two rows are merged into one rather than left as
  separate duplicates.
- **AC-2**: The merge keeps the lower-ID (earlier-created) row as the
  survivor and reassigns every album, track, and gallery photo from the
  other row onto it, then removes the now-empty duplicate row.
- **AC-3**: If the surviving artist already has an album with the same
  title as one being merged in, that album's tracks and covers are folded
  into the existing same-titled album rather than creating a second one.
- **AC-4**: If both merged rows had their own primary gallery image,
  exactly one survives on the merged artist (the one already flagged
  wins; oldest wins on a tie) — never zero, never more than one. (The
  gallery's separate "banner" flag, TDR 016, doesn't exist on `main` yet
  as of this entry; the dedup helper this uses is written generically so
  covering it too is a one-line addition once TDR 016 ships, not a
  redesign.)
- **AC-5**: The same behavior applies to albums that resolve to the same
  MusicBrainz release-group ID.
- **AC-6**: Files already copied to disk are not moved or renamed by a
  merge — only catalog rows change. (No feature in this app renames
  on-disk files after import today; this doesn't add that capability.)
- **AC-7**: A merge failure is logged and does not abort the rest of that
  enrichment run, matching the job's existing per-item error tolerance
  (AC-8 of TDR 003).

Out of scope for this entry: retroactively re-scanning already-enriched
rows created before this ships (their MBIDs were cached but never
cross-checked). Existing duplicates are cleaned up via the manual merge
tool (issue #31 / backlog 018) instead.
