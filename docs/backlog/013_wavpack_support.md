# User Story: WavPack (.wv) Support

## 1. User Value Statement

As a **household member with WavPack-encoded music**, I want to **import
`.wv` files with the same experience as MP3/FLAC** — correct duration,
tags read on import, corrections written back, and hybrid-mode correction
files preserved — So that **WavPack isn't a second-class format I have to
work around**.

## 2. Strict Acceptance Criteria

- **AC-1**: A `.wv` file in an import source is recognized and appears in
  the review plan — today it's silently skipped before tags are even read.
- **AC-2**: A `.wv` track's duration is computed exactly from its WavPack
  block header (matching the precision WAV/FLAC already get), with a
  full-block-scan fallback for the rare case where the header's total
  sample count is unknown upfront (a streaming-encoded file).
- **AC-3**: A `.wv` file's embedded APEv2 tag is read on import —
  Artist/Album/Title/Track Number/Year pre-fill the review plan exactly
  like MP3/FLAC/OGG/M4A already do, instead of always being blank.
- **AC-4**: A `.wv` file's embedded Genre and cover art (APEv2 "Cover Art
  (Front)") are read the same way MP3/FLAC's are — used first, before
  falling back to MusicBrainz/Cover Art Archive enrichment (TDR 003).
- **AC-5**: Corrections made in the review screen (Artist/Album/Title/
  Track Number/Year) are written back into the `.wv` file's APEv2 tag when
  copied, matching what MP3/FLAC tag write-back already does — not just
  stored in the catalog with the file left untouched (today's M4A/OGG
  behavior).
- **AC-6**: If a `.wv` file has a same-named `.wvc` correction file next to
  it (WavPack's hybrid lossy+correction mode), the `.wvc` is copied
  alongside it to the same destination folder, preserving lossless
  reconstruction capability.
- **AC-7**: A `.wvc` companion's destination path is conflict-checked and
  requires the same explicit overwrite consent as any other file — it is
  never silently overwritten.
- **AC-8**: A track with a detected `.wvc` companion shows a small
  indicator on its review-plan row, so it's visible before confirming
  that a second file is being carried along.
- **AC-9**: A `.wvc` file with no matching `.wv` is left alone (not
  recognized as its own track), same as any other non-audio file.
