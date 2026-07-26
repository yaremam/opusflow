# TDR 005: Organize-on-Import

## 1. Context & Architectural Requirements

Per `docs/ARCHITECTURE.md` §3, opusflow's only way to bring music in today is
[TDR 001](001_add_local_directory_design.md)'s model: browse to an existing
folder under a configured `LIBRARY_ROOTS`, register it, and
`library/scan.Scanner` walks it in place — files are read where they sit,
never moved, renamed, or written to. That model assumes the household
already has an organized library sitting on a mounted volume; it has no
answer for "I just downloaded/ripped an album and want opusflow to put it
somewhere sensible."

This feature replaces that model outright (grilled and confirmed: no
dual-path, no migration for anything scanned under the old model — nothing
in production depends on it). From here forward, opusflow is the thing that
organizes your library, not just a reader of one you organized by hand: you
point it at new material, it copies (never touches your originals) and
renames everything into one consistent layout, letting you fix anything the
tags got wrong or left blank before a single byte is copied.

The mockup (grilled and signed off before this doc — six screens: Import
page, choose-a-source, review plan, copying, done, remove-with-files) fixed
the product shape: two ways to name a source (server-folder browse or
client-device upload, converging on one review screen); a review-before-copy
plan with per-track editable fields; explicit conflict blocking; and a
keep-or-delete-files prompt on removal.

## 2. Alternatives Evaluated

### Alternative: relationship to the existing directory/scan model

- **Add alongside (keep TDR 001/002 for already-organized libraries)** —
  Pros: no loss of the "just point at what I already have" case; smaller,
  additive change. Cons: two mental models and two code paths (scan-in-place
  vs. copy-and-organize) to maintain and explain, for a household that in
  practice only ever wants one of them.
- **Replace entirely (chosen)** — Pros: one path in, one thing to explain, no
  in-place-vs-organized distinction for a user to reason about; every track
  in the catalog is guaranteed to live at a predictable path. Cons: a
  household with an already-perfectly-organized library has to re-import
  (i.e. duplicate on disk under `LIBRARY_ROOT`) rather than just pointing at
  it — accepted, since no real library exists under the old model yet.

### Alternative: destination — one `LIBRARY_ROOT` vs. per-artist picked folder

- **Per-artist picked folder** — Pros: matches spreading artists across
  multiple volumes if disk space is a constraint. Cons: a folder-picker step
  on every new artist, and no single place to point backups/permissions at.
- **One configured `LIBRARY_ROOT`, auto-created artist subfolders (chosen)**
  — Pros: zero manual folder setup — importing a new artist's first track
  just works; one path to back up, one place disk usage lives. Cons: all
  organized music must fit on whatever volume `LIBRARY_ROOT` sits on — no
  per-artist spread. `LIBRARY_ROOT` is a new, singular env var, distinct from
  the (renamed, still-plural) source-browsing roots below.

### Alternative: import source — browse-only vs. upload-only vs. both

- **Browse-only (a server-visible staging folder)** — Pros: no upload
  infrastructure, fastest path, reuses `library/roots.go`'s existing
  containment-scoped browsing verbatim. Cons: only works if you can already
  get files onto a volume the server sees — no answer for "these files are
  only on my laptop/phone right now."
- **Upload-only (from the client device)** — Pros: works from anywhere the
  web app is reachable, no server-side staging folder to set up. Cons:
  routes every byte of every import through the browser tab and the app's
  own HTTP connection — meaningfully slower and more fragile for a large
  FLAC library than a same-host file copy, with no benefit for someone who
  could just as easily drop the files onto a mounted volume.
- **Both (chosen)** — Pros: browsing stays the fast default for anything
  already server-visible; upload covers the "it's on my device right now"
  case without forcing everyone through a browser upload for every import.
  Both converge on the same plan/review/copy pipeline — the source is just
  "a directory of files," whether that directory is a browsed path or a
  freshly-populated staging folder. Cons: two source UIs to build (though
  the second, screen 2b, is genuinely new work — a folder/file picker plus
  an upload-progress list — where the first, screen 2a, is a relabel of an
  existing component). Web only for now: the mobile app has no networking
  code yet (per `docs/ARCHITECTURE.md` §3), and building a native
  file/media-picker equivalent is a separable, larger effort.

### Alternative: missing/wrong metadata — auto-placeholder vs. reject vs. manual entry

- **Fall back to "Unknown Artist"/"Unknown Album"** — Pros: reuses the
  existing placeholder rows (TDR 002/003) unchanged, nothing ever fails to
  import. Cons: silently organizes a file under a folder name that doesn't
  describe it — exactly the "messy tags" problem this feature exists to
  solve, just relocated instead of fixed.
- **Reject files with missing required fields** — Pros: nothing
  incorrectly-labeled ever lands in the library. Cons: forces a
  fix-the-source-file-then-retry loop outside the app for anything with
  imperfect tags, which is common (rips, older downloads) rather than rare.
- **Manual entry during review (chosen)** — Pros: the review screen is
  already the natural place to catch and fix exactly this — no separate
  outside-the-app tagging step, no placeholder-named folders. Cons: a large,
  badly-tagged batch means more manual review work per import; accepted
  since it's strictly better than either alternative's failure mode.

### Alternative: when manual entry happens — review-before-copy vs. fix-up-after

- **Import now, fix incomplete items later (a "needs info" queue)** — Pros:
  well-tagged tracks in a batch aren't held up by a few bad ones. Cons:
  introduces a partially-organized on-disk/database state to track and
  surface, and a second UI (the queue) beyond the review screen.
- **Review-before-copy (chosen)** — Pros: nothing is copied until the whole
  plan is complete and conflict-free — no in-between state ever exists on
  disk or in the database. Cons: a large batch with several gaps must be
  fully reviewed before any of it lands, even the already-correct tracks.

### Alternative: copy conflicts — always skip vs. block-until-resolved

- **Always skip existing destination files** — Pros: safe by default, no
  chance of clobbering something. Cons: a legitimate re-import (e.g. fixing
  a bad track number) can't ever overwrite the old copy without manually
  deleting it first — the tool actively works against its own fix-tags
  purpose.
- **Block in the review step, both fix-or-overwrite available (chosen)** —
  Pros: nothing is silently skipped *or* silently clobbered; you see the
  exact conflicting destination path and choose how to resolve it before
  confirming. Cons: one more state the review UI has to render (a
  conflict-row look distinct from a plain warning row).

### Alternative: tag write-back — all formats vs. scoped to mature Go libraries

- **Write back to every supported format (MP3, FLAC, M4A/AAC, OGG, WAV)** —
  Pros: fully honors "the file and its organization always agree," no
  format-shaped exceptions. Cons: the Go ecosystem's tag-*writing* support is
  uneven — solid, actively-maintained libraries exist for ID3v2 (MP3) and
  FLAC Vorbis comments, but MP4/M4A atom writing and Ogg Vorbis-comment
  rewriting have no comparably mature Go library today; building that from
  scratch is a substantial, separate effort with real correctness risk
  (a botched atom/page rewrite can corrupt the file).
- **Scoped to MP3 + FLAC, filename/DB-only for the rest (chosen)** — Pros:
  ships write-back for the two formats where it can be done reliably today,
  without a corruption-risk-laden write path for the others; a household's
  actual correction still lands in the right folder/filename/database
  regardless of format. Cons: an M4A or OGG file's own tags can drift out of
  sync with its folder name after a manual correction — flagged explicitly
  in the UI copy for those tracks rather than silently glossed over, and
  revisitable later if a mature Go MP4/Ogg tag-writer emerges.

## 3. Structural Decision

**Env vars**: `LIBRARY_ROOT` (new, singular) is the write destination —
opusflow creates `<LIBRARY_ROOT>/<Artist>/` on first import for that artist.
The existing plural `LIBRARY_ROOTS` is renamed to `IMPORT_SOURCE_ROOTS`
(same `library.Roots`/`ParseRoots` type and containment logic, unchanged) —
it now scopes *browsing for an import source*, not scan-target registration.

**Schema** (migration 0004, replacing rather than extending): drops
`library_directories` and `library_scan_errors`. Adds `imports` (one row per
*confirmed* import batch — nothing is persisted for a plan still under
review): `id`, `source_description`, `status`
(`copying`/`complete`/`failed`), `files_processed`, `files_total`, `error`,
`created_at`. Adds `import_errors` (`import_id` FK, `path`, `error`) — the
same tolerant per-file-error shape `library_scan_errors` had. `tracks.directory_id`
becomes `tracks.import_id` (provenance — which import batch brought this
track in — not a live scan target); `tracks.path` now always points at the
canonical `<LIBRARY_ROOT>/...` destination. `artists`/`albums` are
unchanged — the destination folder path is always recomputed from
`name`/`year` through one shared sanitizing function, never stored
redundantly.

**Plan generation is stateless until confirm**: browsing or uploading a
source hands the chosen directory to a new `library/organize` package that
reads tags (reusing `scan`'s `ExtractTags`/`DetectFormat`), groups by
detected artist/album, and returns a plan (JSON) to the client — no database
row yet. The client renders and edits that plan entirely in the browser;
confirming POSTs the final, edited plan back. The server re-checks
conflicts at confirm time (something on disk may have changed since the
plan was first generated) before creating the `imports` row and starting the
background copy job — same "runs as a background goroutine, progress
polled" shape `scan.Scanner` already established.

**Upload path**: a new endpoint accepts a multipart upload (preserving each
file's relative path) into a per-session staging directory under a new
scratch location; once every file has arrived, the exact same plan-generation
step described above runs against that staging directory. The staging
directory is removed once the import is confirmed (its files have been
copied to their real destination) or abandoned past a short TTL.

**Copy job**: for each track in the confirmed plan, copies the source file
to its destination path (creating `<Artist>/<Year>.<Album>/` as needed),
writes any manual correction into the destination's own tags via
`github.com/bogem/id3v2` (MP3) or `github.com/go-flac/flacvorbis` +
`github.com/go-flac/go-flac` (FLAC) — both new backend dependencies — and
records the resulting track. A per-file failure is recorded to
`import_errors` and the job continues, mirroring `scan.Scanner`'s
tolerance.

**Removal**: `DELETE /api/library/artists/{id}` and the album equivalent
gain a required `deleteFiles` field; the frontend always shows the
keep-or-delete choice before calling it, never assuming a default.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - New migration 0004: drops `library_directories`/`library_scan_errors`,
    adds `imports`/`import_errors`, renames `tracks.directory_id` to
    `tracks.import_id`.
  - New `backend/internal/library/organize` package (sibling to `scan`,
    `enrich`): plan generation, conflict detection, the copy+rename+tag-write
    job, and the two new Go tag-writing dependencies.
  - `backend/internal/library/scan`: `Scanner`/directory-registration
    machinery removed; `ExtractTags`/`DetectFormat`/duration parsing are
    reused by `organize`, not duplicated.
  - `backend/internal/library/roots.go`: unchanged code, renamed usage
    (`IMPORT_SOURCE_ROOTS` instead of `LIBRARY_ROOTS`) for source browsing.
  - `backend/cmd/server/main.go`: reads `LIBRARY_ROOT` (new) and
    `IMPORT_SOURCE_ROOTS` (renamed); wires `organize` instead of `scan`.
  - `backend/internal/httpserver`: directory-browse/register/list/remove
    endpoints replaced by import endpoints (build plan, confirm, progress);
    artist/album delete endpoints gain `deleteFiles`.
- **`web/`**: `LibraryPage`/`DirectoryPicker`/`DirectoryCard` repurposed into
  the Import page + choose-a-source + review-plan + progress screens from
  the signed-off mockup; artist/album detail pages gain the
  keep-or-delete-files removal prompt.
- **`mobile/`**: unchanged — out of scope per the grilled decision.
- **`docs/`**: this backlog/TDR pair; `docs/ARCHITECTURE.md` §3 (component
  list), §4 (schema), §5 (decision log) updated once implementation lands,
  including removing TDR 001/002's directory-scan description since it no
  longer reflects the system.
