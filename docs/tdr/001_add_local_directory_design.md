# TDR 001: Add Local Directory to Music Library

## 1. Context & Architectural Requirements

This is the first feature built on top of the scaffolded monorepo. Per
`docs/ARCHITECTURE.md`, at the start of this feature: no schema/migrations
exist, no backend code reads `DATABASE_URL`, and the web app has no
routing or state management beyond the default Vite starter screen.

`docs/vision.md` frames the library as a media-server core with direct
filesystem access, similar to Plex/Roon/Sonarr: local folders (host
directories or a mounted NAS share) arrive as Docker volume mounts and are
then "configured as library paths inside the app." Grilling this feature
surfaced requirements beyond that one-liner:

- The household mounts **more than one** independent volume (e.g. a local
  `/music` mount and a separate NAS mount) — a single configured root is
  not sufficient.
- Filesystem browsing exposed through the API/UI must not escape the
  configured mount points — the container filesystem otherwise contains
  unrelated system files.
- Scanning must not block the HTTP request handling a directory add — a
  NAS directory can contain thousands of files and take minutes to walk
  and tag-parse.
- This feature is the first to need persistence, so it's also the one that
  wires up PostgreSQL.
- This feature is the first to need more than one web screen, so it's also
  the one that adds routing to `web/`.

## 2. Alternatives Evaluated

### Alternative: Synchronous scan vs. asynchronous background job

- **Synchronous** — Pros: simplest possible implementation; one
  request/response cycle; trivial to test. Cons: the HTTP request blocks
  for as long as the scan takes (potentially minutes); no way to report
  progress; ties up a request-handling goroutine for the duration.
- **Asynchronous** — Pros: the add-directory call returns immediately;
  the client can poll for live progress; matches real usage (an admin adds
  a directory and moves on, rather than staring at a spinner). Cons:
  requires a job/status model — status, processed count, and total count
  need to be tracked and persisted per directory, not just a single
  request/response.
- **Chosen: asynchronous.** A goroutine performs the scan; status,
  processed-file count, and total-file count live as columns on the
  library directory's row, polled by the client. No separate queue or
  worker process — at single-household, single-process scale, an in-process
  goroutine is sufficient and avoids standing up infrastructure (e.g. a job
  queue) nothing else needs yet.

### Alternative: Free-text path input vs. server-side directory browser/picker

- **Free-text input** — Pros: no new endpoint, trivial UI (one text
  field). Cons: the user has to know the exact in-container path and can
  easily typo it; nothing stops them from guessing a path outside any
  mounted volume; doesn't help them discover what's actually mounted.
- **Server-side browser** — Pros: discoverable (the user sees what's
  actually there); naturally enforces the "configured roots only" boundary
  because the picker simply never renders anything outside them. Cons:
  needs a new "list subdirectories of a path" endpoint and a picker UI
  component (root selector + breadcrumb + folder list).
- **Chosen: server-side browser**, scoped to `LIBRARY_ROOTS`.

### Alternative: single configured root vs. multiple configured roots

- **Single root** — Pros: simplest config (one env var, one path) and
  simplest picker (no root selector). Cons: cannot represent more than one
  independent volume mount, which the household's actual setup requires.
- **Multiple roots** — Pros: matches real deployment — `docker-compose.yml`
  can mount several host paths, each becoming its own selectable root.
  Cons: config format has to represent a list, and the picker needs a root
  selector step before browsing.
- **Chosen: multiple roots**, via `LIBRARY_ROOTS` — a comma-separated list
  of absolute paths, consistent with how existing config (`PORT`,
  `STATIC_DIR`) is a single flat env var rather than a structured config
  file. The picker presents each root as a tab/segmented control; browsing
  and directory registration are only ever resolved relative to one of
  these roots, and any path that does not resolve under one of them is
  rejected by the backend regardless of what the UI sends.

### Alternative: per-file scan errors — fail-fast vs. skip-and-continue

- **Fail-fast** — Pros: simpler state machine — a scan is either fully
  complete or failed, nothing in between. Cons: one corrupt or unreadable
  file blocks importing every other file in the directory — real-world
  libraries reliably contain a handful of problem files, so this would
  make most large-directory scans fail outright.
- **Skip-and-continue** — Pros: maximizes usable library data from a
  single scan; matches real-world tagging inconsistency (a few bad files
  shouldn't cost the other thousands). Cons: needs a place to record and
  surface per-file errors alongside an otherwise-successful scan.
- **Chosen: skip-and-continue.** A directory's status is `failed` only for
  a job-level failure (the registered directory itself becomes
  inaccessible mid-scan) — never for isolated per-file errors, which are
  recorded and surfaced but leave the directory `complete`.

## 3. Structural Decision

Build this as: a backend-owned library-directory registry (Postgres-backed)
with an async scan worker (in-process goroutine, no external queue), a
filesystem-browse endpoint scoped to `LIBRARY_ROOTS`, and a scanning engine
that recursively walks a registered directory, identifies audio files by
extension (`.mp3`, `.flac`, `.m4a`, `.aac`, `.ogg`, `.wav`), and extracts
tags per format, skipping and recording individual file failures rather
than aborting. The web app gets its first router and a "Library" page that
lists directories (status/progress/track count/remove) and hosts the
add-directory picker (root tabs → breadcrumb folder browser → confirm).

This is deliberately the feature that pays off two pieces of deferred
groundwork noted in `docs/ARCHITECTURE.md` §6: Postgres wiring and the
web app's router/state library choice — both were explicitly deferred
"until the first feature needs" them, and this is that feature.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - New Postgres wiring: connection pool reading `DATABASE_URL`, plus a
    migrations mechanism (none exists yet).
  - New tables: `library_directories` (root, path, status, processed
    count, total count, timestamps, last error) and `tracks` (foreign key
    to directory, path, title, artist, album, track number, year, genre,
    duration).
  - New internal packages: filesystem browser (path-containment check
    against `LIBRARY_ROOTS`), audio tag scanner, async scan job runner.
  - New HTTP endpoints (indicative, exact routes decided at implementation
    time): list configured roots, browse a path's subdirectories, list
    registered directories (with status), add a directory, remove a
    directory.
  - New config: `LIBRARY_ROOTS` env var, alongside existing `PORT` /
    `STATIC_DIR`.
- **`web/`**:
  - First real screen added to the app. Resolved at implementation time
    without a router: the directory picker is an in-page modal on the
    Library page, not a second route, so the router/state-library choice
    noted as deferred in `docs/ARCHITECTURE.md` §6 stays deferred.
  - New "Library" page and directory-picker component; a polling client
    for scan status.
- **`mobile/`**: out of scope. The mobile app has no networking code yet
  (`docs/ARCHITECTURE.md` §6) and the vision doc does not require this
  feature to reach mobile in v1.
- **Schema**: first migration(s) in the project, creating
  `library_directories` and `tracks`.
- Update `docs/ARCHITECTURE.md` §3–6 once implementation lands: move
  Postgres wiring and the web router choice out of §6 (deferred work) and
  into §2/§3 as decided, add the two new tables to §4, and add a row to
  the §5 decision log linking back to this TDR.
