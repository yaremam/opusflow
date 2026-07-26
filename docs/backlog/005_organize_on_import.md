# User Story: Organize-on-Import

## 1. User Value Statement

As a **household member adding new music to opusflow**, I want **opusflow to
copy, rename, and organize files into a consistent artist/album/track folder
structure as I bring them in — from a folder the server can already see, or
uploaded straight from my own device — reviewing and filling in anything the
tags don't already tell it before anything is copied**, So that **my library
stays uniformly organized on disk without me manually renaming or moving
files myself, and incomplete or wrong tags never make it into the library
unresolved**.

## 2. Strict Acceptance Criteria

### Replacing the old model

- **AC-1**: The existing "add a directory" flow (browse and register an
  arbitrary host folder, scan its files in place; TDR 001/002) is removed.
  From this feature forward, the only way music enters the library is the
  import flow below. No migration of directories/tracks registered under the
  old model is provided.
- **AC-2**: A single configured `LIBRARY_ROOT` is where opusflow stores every
  file it organizes. Importing a track for an artist that has no folder yet
  creates `<LIBRARY_ROOT>/<Artist>/` automatically — there is no folder
  picker or manual per-artist setup step.
- **AC-3**: Every imported file lands at
  `<LIBRARY_ROOT>/<Artist>/<Year>.<Album>/<NN>.<Title>.<ext>`, `NN` being the
  track number zero-padded to two digits.

### Choosing a source

- **AC-4**: Starting an import offers two source options: browsing a folder
  the server can already see (scoped to a configured set of importable
  source roots, the same containment model today's directory browsing
  uses), or uploading a folder/files directly from the user's own device
  (web only — no mobile support in this feature).
- **AC-5**: A client-side upload is written to a server-side staging area
  before anything else happens. From that point on, it is processed
  identically to a server-browsed folder — same plan generation, same
  review screen, no separate code path a user would notice.

### Building and reviewing the plan

- **AC-6**: Once a source is chosen, opusflow reads tags from every audio
  file found under it and builds a plan grouped by detected album: artist,
  album title, year, and each track's number/title, together with the
  destination path each track would land at. Nothing is copied at this
  point.
- **AC-7**: Any field the plan can't determine from tags is left blank and
  editable rather than defaulted to a placeholder ("Unknown Artist", etc.)
  or silently guessed — the user fills it in directly on the review screen.
- **AC-8**: Any planned destination path that already exists on disk is
  flagged as a conflict and blocks confirmation for that track until the
  user either edits the conflicting field (e.g. corrects a wrong track
  number) or explicitly chooses to overwrite the existing file.
- **AC-9**: Confirming the import is disabled while any track in the plan is
  missing a required field or has an unresolved conflict.

### Copying

- **AC-10**: Confirming the plan starts a background job — progress visible
  the same way today's directory-scan progress is — that copies (never
  moves or deletes) each source file to its destination path.
- **AC-11**: Any manual correction made during review is also written into
  the *copied* file's own embedded tags for formats with mature Go
  tag-writing support (MP3 via ID3v2, FLAC via Vorbis comments). For other
  supported audio formats (M4A/AAC, OGG, WAV), the correction still applies
  to the destination filename, folder, and database record; embedded-tag
  write-back for those formats is not guaranteed in this version.
- **AC-12**: A per-file copy failure is tolerated and recorded (the same
  skip-and-continue pattern as today's scan file errors) rather than
  aborting the rest of the import.

### Removing music

- **AC-13**: Removing an artist or album always asks, in the moment, whether
  to also delete its copied files under `LIBRARY_ROOT` or only remove it
  from the catalog — there is no silent default either way.
