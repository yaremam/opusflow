# User Story: Add Local Directory to Music Library

## 1. User Value Statement

As a **household admin setting up opusflow**, I want to **browse the folders
mounted into the container and add one as a library directory that gets
scanned into my music library**, So that **my local music files show up as
tracks in opusflow without me having to know or type exact filesystem
paths**.

## 2. Strict Acceptance Criteria

- **AC-1**: The backend is configured with one or more library roots via a
  `LIBRARY_ROOTS` env var (comma-separated absolute paths, one per Docker
  volume mount). A user can list the immediate subdirectories of any path
  under a configured root. Any path outside all configured roots is
  rejected and cannot be listed or browsed.
- **AC-2**: A user can register a directory (any path under a configured
  root, at any depth) as a library directory. Registering a path that
  exactly matches an already-registered directory is rejected with a clear
  error. Registering a path that is a parent or subdirectory of an existing
  registered directory is allowed.
- **AC-3**: Registering a directory returns immediately, without waiting
  for the scan to finish, and starts a background scan job. The directory's
  status is `scanning` until that job finishes.
- **AC-4**: While a directory is `scanning`, its status exposes both the
  number of files processed so far and the total number of matching audio
  files found, so a client can display progress like "206 of 500 files
  scanned."
- **AC-5**: The scan recursively walks the registered directory and its
  subdirectories, identifying files with extensions `.mp3`, `.flac`,
  `.m4a`, `.aac`, `.ogg`, and `.wav` as tracks to import.
- **AC-6**: For each identified audio file, the scan extracts `title`,
  `artist`, `album`, `track number`, `year`, and `genre` from the file's
  tags where present. If a file has no readable title tag, `title` falls
  back to the filename with its extension stripped.
- **AC-7**: If an individual file cannot be processed (unreadable,
  permission error, corrupt or unparseable tags), the scan records that
  file's path and an error message, then continues scanning the remaining
  files. A single bad file never aborts the whole scan.
- **AC-8**: When a scan finishes having successfully walked the directory
  (even if some individual files failed per AC-7), the directory's status
  becomes `complete`, exposing the count of imported tracks and the list of
  any per-file errors.
- **AC-9**: If the scan cannot proceed at all — for example the registered
  directory itself becomes unreadable or disappears during the scan — the
  directory's status becomes `failed`, with an error describing why. This
  is distinct from AC-7/AC-8: isolated per-file errors do not produce a
  `failed` directory.
- **AC-10**: A user can remove a registered library directory. Removal
  deletes the directory's registration and hard-deletes (cascades to) all
  track records that were imported from it. No confirmation-dialog or
  soft-delete/undo behavior is required.
- **AC-11**: The web app has a "Library" page that lists every registered
  directory with its current status (`scanning`/`complete`/`failed`), live
  progress while scanning, the imported track count once complete, and a
  control to remove each directory. The page also provides an entry point
  (an "Add directory" action) that opens a picker: select a configured
  root, browse its subdirectories, and confirm adding the selected folder.
