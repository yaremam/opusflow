# opusflow Architecture

Living reference for the system as a whole — components, interfaces, frameworks,
schema, and the load-bearing decisions behind them. Per-feature rationale in full
(alternatives considered, pros/cons) lives in [`docs/tdr/`](tdr/); this document
summarizes and links out rather than duplicating it. Update this file whenever a
feature changes a component boundary, adds a table, or reverses an earlier
decision — it should always describe the system as it is today, not as it was
designed.

**Status**: organize-on-import ([TDR 005](tdr/005_organize_on_import_design.md))
replaced the original add-directory/scan-in-place model
([TDR 001](tdr/001_add_local_directory_design.md)) — the library is now built
by importing from a chosen source (a browsed server folder or an upload),
which copies files into a library's organized root, renamed into
`<Artist>/<Year>.<Album>/<NN>.<Title>` and reviewed before anything is
copied. [TDR 006](tdr/006_multiple_libraries_design.md) then turned "the
library root" into a real in-app concept — a **library** (name + root
folder) created and managed from the app itself, with more than one able to
exist side by side — removing the `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS`
environment variables entirely; filesystem browsing (for both an import
source and a new library's root, which can now also be created on the spot
rather than needing to already exist) is confined to `DATA_DIR` when
configured (`/data` in `deploy/docker-compose.yml`), or unrestricted from
`/` when it isn't (e.g. a plain `go run` with no such mount to confine
anything to). On top of that: a Home screen plus Artist/Album/Song browsing
([TDR 002](tdr/002_home_and_browsing_design.md)), artist/album artwork plus
MusicBrainz/Wikipedia-sourced facts and bio
([TDR 003](tdr/003_artwork_and_info_design.md)), and self-hosted deployment
via a nightly prebuilt image ([TDR 004](tdr/004_self_hosted_deployment_design.md)).
Mobile is still an untouched Expo starter.

## 1. System context

```mermaid
flowchart TB
    Browser["Browser"]
    Phone["Android / iOS app<br/>(React Native, Expo)"]

    subgraph Container["Docker image (single container)"]
        Backend["Go backend<br/>net/http, cmd/server"]
        Web["Web app static build<br/>(React + Vite, served from /app/web)"]
    end

    Postgres[("PostgreSQL")]
    DataVolume[("Data volume<br/>(one broad read-write mount)")]
    Artwork[("Artwork cache volume<br/>(ARTWORK_DIR)")]
    External[("MusicBrainz /<br/>Cover Art Archive /<br/>Wikidata + Wikipedia")]

    Browser -->|HTTP| Backend
    Backend -->|serves static files,<br/>SPA fallback to index.html| Web
    Phone -->|HTTP, not yet defined| Backend
    Backend -->|libraries, imports,<br/>import_errors, tracks,<br/>artists, albums| Postgres
    Backend -->|browse (source/library root),<br/>write (import), delete (remove)| DataVolume
    Backend -->|read/write resized<br/>artist photos, album covers| Artwork
    Backend -->|HTTPS, rate-limited| External
```

The backend and web app are built and shipped as **one** Docker image (root
`Dockerfile`): a multi-stage build compiles the Go binary and the Vite static
build separately, then copies both into a minimal `distroless` runtime image.
The Go binary serves the API and the static web build from the same process —
there is no separate web server/container. The mobile app is a native
Android/iOS build (via Expo), not part of this image.

That same image is also published, prebuilt, to `ghcr.io/yaremam/opusflow`
by a nightly CI pipeline ([TDR 004](tdr/004_self_hosted_deployment_design.md)),
for self-hosting without a Go/Node toolchain on the target machine — see
[`deploy/`](../deploy/) and [`docs/deploy/synology.md`](deploy/synology.md).

## 2. Core stack

| Concern | Choice | Notes |
|---|---|---|
| Backend | Go 1.24, stdlib `net/http` | `backend/cmd/server` + `backend/internal/httpserver`; no router library yet — Go 1.22+'s `ServeMux` method+path patterns cover current needs |
| Web frontend | React 19 + Vite + TypeScript + `react-router` | `web/`; router added by [TDR 002](tdr/002_home_and_browsing_design.md) — no separate state-management library yet |
| Mobile | Expo (React Native) + TypeScript | `mobile/`, default `create-expo-app blank-typescript` template, not yet customized |
| Database | PostgreSQL 17 | `backend/internal/db` (`lib/pq` driver, hand-rolled embed-based migration runner — no ORM/migration-library dependency); wired up by [TDR 001](tdr/001_add_local_directory_design.md) |
| Audio tag reading/duration parsing | `github.com/dhowden/tag` (reading) + hand-rolled per-format duration parsers | `backend/internal/library/scan` (format detection + duration only) and `backend/internal/library/organize` (plan-building tag reads); see [TDR 005](tdr/005_organize_on_import_design.md) |
| Audio tag writing | `github.com/bogem/id3v2` (MP3) + `github.com/go-flac/go-flac` + `flacvorbis` (FLAC) | `backend/internal/library/organize`; scoped to MP3/FLAC only — no mature Go writer exists for M4A/OGG/WAV, see [TDR 005](tdr/005_organize_on_import_design.md) |
| Artwork/info sourcing | MusicBrainz + Cover Art Archive + Wikidata/Wikipedia (no API key, descriptive `User-Agent`) | `backend/internal/library/enrich`; `golang.org/x/image/draw` for resizing; see [TDR 003](tdr/003_artwork_and_info_design.md) |
| Package manager (web/mobile) | pnpm workspaces, pinned via corepack (`pnpm@9`), Node.js 24 | root `pnpm-workspace.yaml` covers `web/` and `mobile/`; `Dockerfile`'s `node:24-alpine` build stage and the nightly workflow's `setup-node` both pin the same version |
| Packaging | Docker (see §1) | root `Dockerfile` + `docker-compose.yml` (local build/dev) |
| Deployment | Nightly multi-platform (amd64+arm64) image on GHCR, `.github/workflows/nightly.yml` | `deploy/docker-compose.yml` pulls it instead of building; see [TDR 004](tdr/004_self_hosted_deployment_design.md) |

## 3. Components

- **`backend/cmd/server`** — process entrypoint. Reads `PORT` (default
  `8080`), `STATIC_DIR` (set to `/app/web` in the Docker image; empty in
  local `go run`, which then serves API-only), `ARTWORK_DIR` (optional,
  like `STATIC_DIR` — unset disables embedded-art saving and the enrichment
  job entirely rather than erroring), and `DATA_DIR` (optional — set to
  `/data` in the Docker image, confining filesystem browsing and
  library-root creation to under it; unset means unrestricted from `/`,
  e.g. a plain `go run` with nothing to confine anything to). There is no
  `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS` env var (removed by
  [TDR 006](tdr/006_multiple_libraries_design.md)) — a library's root is
  created and stored in the database from within the app, and can now be
  a brand-new folder rather than one that has to already exist. The
  database connection is `DATABASE_URL` if set (an explicit
  connection string), otherwise built from `POSTGRES_HOST`/`POSTGRES_PORT`
  (defaulting to the compose service name and `5432`) plus
  `POSTGRES_USER`/`POSTGRES_DB` and an optional `POSTGRES_PASSWORD` —
  `deploy/docker-compose.yml` sets no password at all: `postgres` isn't
  reachable outside the compose network, so it runs with
  `POSTGRES_HOST_AUTH_METHOD=trust` instead, and there's nothing
  credential-shaped to keep in sync between the two services or leak into
  the file. Opens Postgres, runs migrations, wires
  up `library.Service` (and, when `ARTWORK_DIR` is set, an `enrich.Job` run
  once at startup after migrations succeed — TDR 003's backfill trigger)
  before starting the HTTP server.
- **`backend/internal/httpserver`** — builds the root `http.Handler`:
  `GET /health` (reports `{"status":"ok","revision":"<GIT_SHA>"}` — `revision`
  is omitted when unset, e.g. local `go run`; stamped into the Docker image
  by the nightly pipeline, [TDR 004](tdr/004_self_hosted_deployment_design.md));
  the import and library endpoints (below); when `ARTWORK_DIR` is set, a
  static file server for it under `/artwork/`; and, when `STATIC_DIR` is
  set, a static file server for the built web app with SPA fallback
  (unmatched GETs serve `index.html` rather than 404, so client-side routing
  survives a refresh).
  - `GET /api/libraries` — list every library (name, root path, track count)
  - `POST /api/libraries` `{"name", "rootPath"}` — create a library;
    `rootPath` must already exist as a directory and fall within `DATA_DIR`
    if one is configured
  - `DELETE /api/libraries/{id}?deleteFiles=true|false` — remove a library
    and everything imported into it; `deleteFiles` is required, never
    defaulted
  - `GET /api/imports` — list every recorded import, newest first
  - `GET /api/imports/browse?path=` — list a path's immediate subdirectories
    (confined to `DATA_DIR` if configured, unrestricted from `/` otherwise;
    used for both import-source and create-a-library browsing)
  - `POST /api/imports/browse/folders` `{"parentPath", "name"}` — create a
    new subdirectory directly inside `parentPath` (one level, never nested)
    and return it as a browsable entry; same `DATA_DIR` confinement as
    `GET .../browse`; idempotent if the folder already exists
  - `POST /api/imports/plan` `{"libraryId", "sourceDir"}` — build a plan by
    reading tags under a browsed source directory, computed against that
    library's root; rejects a source the same as or nested inside any
    library's root (TDR 006 AC-8)
  - `POST /api/imports/plan/validate` `{"libraryId", "plan"}` — recompute
    destinations/conflicts/missing-field errors for a reviewer-edited plan
  - `POST /api/imports/upload` (multipart, `libraryId` field) — stage an
    uploaded folder, then build a plan from it the same way
  - `POST /api/imports` `{"libraryId", "sourceDescription", "plan"}` —
    validate one last time, then copy in the background; `202` + the new
    import, or `422` + validation errors if the plan still isn't ready
  - `GET /api/imports/{id}` — an import's copy progress and any per-file
    errors
  - `GET /api/library/artists` / `GET /api/library/albums` /
    `GET /api/library/songs` — paginated, sorted (`recent`/`name`), filtered
    (`genre`, `year`, free-text `q`) listings; see
    [TDR 002](tdr/002_home_and_browsing_design.md)
  - `GET /api/library/artists/{id}` — artist detail (with their albums)
  - `DELETE /api/library/artists/{id}?deleteFiles=true|false` — remove an
    artist and its albums/tracks; `deleteFiles` is required, never defaulted
  - `POST /api/library/artists/{id}/art/retry` — reset the artist's art
    status to pending and wake the enrichment job immediately (TDR 007);
    `202` + the artist as of right now, for the frontend to poll
  - `POST /api/library/artists/{id}/art` (multipart, one `image` field,
    8MB cap) — save a manually-uploaded photo, bypassing MusicBrainz/Cover
    Art Archive entirely (TDR 007); `200` + the updated artist, or `503` if
    `ARTWORK_DIR` isn't configured
  - `GET /api/library/albums/{id}` — album detail (with its track listing)
  - `DELETE /api/library/albums/{id}?deleteFiles=true|false` — remove an
    album and its tracks
  - `POST /api/library/albums/{id}/art/retry` / `POST
    /api/library/albums/{id}/art` — album-flavored counterparts of the two
    artist routes above
- **`backend/internal/db`** — Postgres connection (`Open`) and schema
  migrations (`Migrate`, embedding `internal/db/migrations/*.sql`, tracked in
  a `schema_migrations` table).
- **`backend/internal/library`** — the library domain: `Browse` (a plain
  directory listing), `ValidateDirectory` (confirms a path exists as a
  directory), `CreateFolder` (makes a new one-level subdirectory, so a
  library's root no longer has to already exist), and `WithinRoot`/
  `ErrOutsideRoot` (the `DATA_DIR` containment check `Service` applies to
  all three via `SetBrowseRoot` — a no-op when unset, TDR 006's original
  unrestricted stance), `Store` (Postgres persistence for
  libraries/imports/tracks/import errors/artists/albums; direct artist/
  album/library deletion with an optional on-disk file delete — deleting a
  library also sweeps any artist/album left with zero tracks, the one place
  that orphan-cleanup shape survives after TDR 005 removed it as a general
  behavior; plus the enrichment query/update methods satisfying
  `enrich.Store` — `SetArtistArt`/`SetAlbumArt` only overwrite the
  photo/cover path columns on a `Found` write, so a `NotFound`/`Failed`
  outcome never nulls out a previously-found image, the fix TDR 007's
  always-available retry depends on; `ResetArtistArt`/`ResetAlbumArt` mark
  status pending again without touching those paths), `Service`
  (orchestrates library create/list/delete, browse/plan/validate/confirm
  scoped to a chosen library, and artist/album/song listing/detail/delete;
  `ConfirmImport` starts the copy as a background goroutine so
  `POST /api/imports` returns before it finishes, then runs one
  `enrich.Job` pass under a fresh background context once the copy
  completes — the same post-scan trigger shape TDR 003 introduced;
  `RetryArtistArt`/`RetryAlbumArt` (TDR 007) reuse that same
  wake-the-job-in-the-background shape on demand, and
  `UploadArtistArt`/`UploadAlbumArt` save a manually-chosen image straight
  through the same `enrich.ImageStore` the job uses, bypassing MusicBrainz/
  Cover Art Archive entirely).
- **`backend/internal/library/organize`** — the organize-on-import engine
  (TDR 005): `BuildPlan` (recursive source walk, tag reads that leave a
  blank field blank rather than guessing — deliberately distinct from
  `scan.ExtractTags`'s old filename-fallback behavior), `Validate`
  (recomputes each track's canonical `<Artist>/<Year>.<Album>/<NN>.<Title>`
  destination and on-disk conflict status against the plan's current,
  possibly reviewer-edited values — the server, never the client, decides
  what confirming would actually do), and `Copy` (copies each file to its
  destination, writes the plan's corrected fields back into the copy's own
  MP3/FLAC tags, re-extracts genre/embedded artwork from the original tags
  for the catalog, and tolerates per-file failure without aborting the rest
  — the same tolerance `scan.Scanner` used for the model this replaced).
- **`backend/internal/library/scan`** — now scoped to what `organize` still
  needs: audio format detection by extension (mp3/flac/m4a/aac/ogg/wav) and
  per-format duration parsing. The old recursive-scan-in-place engine
  (`Scanner`, `Track`, filename-fallback tag extraction) was removed with
  TDR 005; TDR 001's original design remains in that TDR's write-up for
  history.
- **`backend/internal/library/scan/duration`** — per-format audio duration:
  exact for WAV/FLAC/MP4, best-effort for OGG (last page granule position)
  and MP3 (Xing/Info VBR header if present, else a bitrate/filesize estimate).
- **`backend/internal/library/enrich`** — the background artwork/info
  worker (TDR 003): `MusicBrainz` (search + lookup, rate-limited to its
  usage policy), `CoverArtArchive` (album covers by release-group MBID),
  `Wikidata` (resolves a MusicBrainz "wikidata" relation into a Commons
  photo filename and an English Wikipedia article title, then fetches
  each), `ImageStore` (resizes into thumb/full JPEG variants under
  `ARTWORK_DIR`), and `Job` (orchestrates all of the above against a
  `Store` interface `*library.Store` satisfies — same "leaf package,
  dependency points one way" shape as `scan`/`library.Store.InsertTrack`).
  `Job.Run` processes every artist/album with any of art/facts/bio still
  `pending`/`failed`, not scoped to a particular scan, tracking each kind's
  outcome independently.
- **`web/`** — `react-router` routes wrapped in `src/components/AppLayout.tsx`
  (persistent Home/Artists/Albums/Songs/Import/Libraries header nav):
  `src/pages/HomePage.tsx` (library snapshot + recently-added previews, with
  a grid/table toggle over the artists/albums previews — TDR 008,
  remembered in `localStorage`, the app's first persisted UI preference),
  `src/pages/ArtistsPage.tsx` / `AlbumsPage.tsx` / `SongsPage.tsx` (paginated,
  sortable, filterable index pages, sharing fetch/pagination logic via
  `src/hooks/useListPage.ts`), `src/pages/ArtistDetailPage.tsx` /
  `AlbumDetailPage.tsx` (each with a "Remove…" action opening
  `src/components/RemoveModal.tsx` — TDR 005's keep-vs-delete-files prompt,
  reused verbatim by `LibrariesPage` below), `src/pages/ImportPage.tsx` — the
  organize-on-import flow as one page-level state machine (history list →
  choose/create a library → choose a source → browse/upload → review plan →
  copying → done), replacing the old directory-list Library page, and
  `src/pages/LibrariesPage.tsx` (TDR 006) — lists every library and is the
  only place one can be deleted from. Both pages compose
  `src/components/SourceFolderPicker.tsx` (breadcrumb folder browser,
  confined to `DATA_DIR` when the backend has one configured; an optional
  `nameField` prop lets it double as the create-a-library form, in which
  case a "＋ New folder" control also appears, letting a library's root be
  a brand-new folder rather than one that has to already exist) and, for
  uploads,
  `src/components/UploadDropzone.tsx` (drag-and-drop via the File System
  Entry API, or a `webkitdirectory` file input; per-file progress derived
  from each file's byte offset within one combined upload request, since the
  backend accepts the whole folder as a single multipart POST). The review
  step edits plan fields inline and revalidates against the server on blur —
  see `validatePlan` in `src/api/library.ts` — never trusting client-computed
  destinations or conflict state. `src/components/ArtTile.tsx` (real artwork
  vs. a refined placeholder glyph) and `src/components/InfoBlock.tsx`
  (facts-chip row + optional bio/description) are shared across every page
  that renders artist/album art (TDR 003). `src/api/library.ts` is the typed
  fetch client.
- **`mobile/`** — untouched Expo starter. No navigation or API client added
  yet.

## 4. Data model

Postgres, migrated via `backend/internal/db` (see §3):

- **`libraries`** — one row per library (TDR 006): `name`, `root_path`,
  `created_at`. Created and managed entirely from the app — there is no
  `LIBRARY_ROOT` environment variable equivalent. Deleting one cascades to
  its imports (see below) and sweeps any artist/album left with zero tracks.
- **`imports`** — one row per confirmed import run: `library_id` (FK,
  `ON DELETE CASCADE` — which library this import copied into),
  `source_description` (the browsed source path, or "Uploaded from
  device"), `status` (`copying` / `complete` / `failed`), `files_processed`,
  `files_total`, `error`, `created_at`. Replaces TDR 001's
  `library_directories` — an import is a historical record of a copy that
  happened, not a managed resource with its own remove action (see TDR 005);
  it's also not itself browsable by library — catalog browsing stays
  unified across every library (TDR 006 AC-2).
- **`tracks`** — one row per copied audio file: `import_id` (FK,
  `ON DELETE CASCADE`; renamed from `directory_id`), `path` (now the
  organized destination path, not the original source path), `title`,
  `artist`, `album`, `track_number`, `year`, `genre`, `duration_seconds`,
  plus `artist_id`/`album_id` (FKs, `ON DELETE CASCADE` — added by TDR 005
  so an artist/album can be deleted directly, not only via its import).
- **`import_errors`** — one row per file an import couldn't copy:
  `import_id` (FK, `ON DELETE CASCADE`), `path`, `error`. Replaces TDR 001's
  `library_scan_errors`; isolated file errors don't change the import's
  `status` away from `complete` — only every file failing sets
  `status = 'failed'`.
- **`artists`** — one row per distinct artist name (including a real,
  empty-name "Unknown Artist" row for untagged tracks), `name` unique. Added
  by [TDR 002](tdr/002_home_and_browsing_design.md); upserted as tracks are
  copied in. TDR 001's orphan-sweep-on-directory-removal cleanup is gone —
  TDR 005 replaced it with direct `DELETE /api/library/artists/{id}`, which
  cascades to the artist's own albums/tracks (and, with `deleteFiles=true`,
  their files on disk) rather than a global "no tracks left anywhere" sweep.
  [TDR 003](tdr/003_artwork_and_info_design.md) added `musicbrainz_id`
  (cached once matched), `photo_thumb_path`/`photo_path` + `art_status`,
  `formed_year`/`country`/`genres` + `facts_status`, and `bio`/
  `bio_source_url` + `bio_status` — three independent `enrich_status`
  (`pending`/`found`/`not_found`/`failed`) columns, one per kind, rather
  than a single combined status.
- **`albums`** — one row per `(title, artist_id)` pair, `year`. Same
  upsert/direct-delete lifecycle as `artists` (`DELETE
  /api/library/albums/{id}`), and the same TDR 003 extension shape:
  `musicbrainz_id`, `cover_thumb_path`/`cover_path` + `art_status`,
  `label`/`country`/`genres` + `facts_status`, `description`/
  `description_source_url` + `description_status`.

## 5. Feature-by-feature decision log

Full write-ups (alternatives evaluated, pros/cons) are in `docs/tdr/`; this is
an index with the one-line "why" for each, newest first.

| Feature | TDR | Chosen approach | Why (one line) |
|---|---|---|---|
| Home page table view | [008](tdr/008_home_page_table_view_design.md) | A shared "▦ Grid / ☰ Table" toggle above the home page's Recently added artists/albums sections; table columns (Artist/Albums/Songs, Album/Artist/Year) sourced entirely from fields the existing API responses already carry; choice remembered in `localStorage`, no backend change | GitHub issue #8; a large library benefits from scanning more rows at once with more detail per row than the card/chip grid shows, without giving up the grid for people who prefer it |
| Artwork status, manual retry & upload | [007](tdr/007_artwork_retry_and_upload_design.md) | Art status (pending/found/not_found/failed) exposed via the API and surfaced as a badge/pill wherever it's rendered; "Retry lookup" resets status to pending and wakes the background job immediately, always available even on a `found` item; "Upload photo/cover" bypasses MusicBrainz/Cover Art Archive entirely, synchronous, always available; `SetArtistArt`/`SetAlbumArt` fixed to only overwrite path columns on a `Found` write | The API previously couldn't distinguish "still looking" from "gave up," and a `failed`/`not_found` item had no way to be nudged short of a new import; scoped to Art only, Facts/Bio/Description unchanged |
| Multiple libraries | [006](tdr/006_multiple_libraries_design.md) | `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS` env vars removed; a library (name + root folder, now creatable on the spot rather than needing to pre-exist) is created/deleted from within the app, several can exist; catalog browsing stays unified across all of them; filesystem browsing confined to `DATA_DIR` when configured (unrestricted from `/` otherwise — amended post-TDR-006, see `DATA_DIR` above); deleting a library cascades with the same keep-or-delete-files choice as artist/album removal | A fixed, deploy-time destination folder was inflexible for more than one logical collection, and coupled a purely operational choice to a redeploy rather than something changeable in the app |
| Organize-on-import *(its single-`LIBRARY_ROOT` destination superseded by [006](tdr/006_multiple_libraries_design.md))* | [005](tdr/005_organize_on_import_design.md) | Replaces add-directory/scan-in-place entirely: import copies files from a chosen source into a single `LIBRARY_ROOT`, renamed into `<Artist>/<Year>.<Album>/<NN>.<Title>`; review-before-copy with server-computed destinations/conflicts; tag write-back scoped to MP3/FLAC; direct artist/album deletion with explicit keep-or-delete-files choice | The original scan-in-place model left files wherever they started, with no consistent on-disk naming — organizing them is the point, not an optional extra step |
| Self-hosted deployment | [004](tdr/004_self_hosted_deployment_design.md) | Nightly multi-platform image on GHCR (skip-if-unchanged, test-gated); separate `deploy/docker-compose.yml` pulling it, with bundled Postgres and multi-root music bind-mounts | Removes the Go/Node toolchain requirement from the target machine (a NAS); mirrors a proven pattern from a sibling project (docuflow) adapted for opusflow's host-mounted-library model |
| Artist/album artwork and info *(art status/manual override amended by [007](tdr/007_artwork_retry_and_upload_design.md))* | [003](tdr/003_artwork_and_info_design.md) | Embedded-tag art first, MusicBrainz + Cover Art Archive + Wikidata/Wikipedia fallback via a background `enrich.Job`; three independent per-kind statuses; files on disk under `ARTWORK_DIR`, not DB blobs | Free/open/no-API-key sources matching the project's anti-proprietary-protocol stance; a background job (not inline with scanning) respects MusicBrainz's rate limit and doubles as backfill for pre-existing libraries |
| Home screen & library browsing | [002](tdr/002_home_and_browsing_design.md) | Normalized `artists`/`albums` tables (upserted at scan/import time); numbered pagination; `react-router` added | Gives Artist/Album detail pages stable IDs to route by and a natural home for future streaming-service artist linkage |
| Add local directory to library *(superseded by [005](tdr/005_organize_on_import_design.md))* | [001](tdr/001_add_local_directory_design.md) | Async goroutine-based scan; server-side directory picker scoped to multiple `LIBRARY_ROOTS`; skip-and-continue per-file error handling | Matches real multi-volume households and real-world tagging inconsistency without over-building (no job queue, no router) — superseded once organizing files on disk became a requirement, not just cataloging them in place |

## 6. Deferred / future work

- **API contract between backend and mobile app** — not designed yet; the
  mobile app has no networking code.
- **Full artist/album/song browsing beyond the current index+detail pages** —
  no playback, no artist-following (per `docs/vision.md`); a song's only
  navigation target today is its album.
- **Artwork match correctness** — the enrichment job still accepts
  MusicBrainz's top search result with no confidence threshold (TDR 003);
  an ambiguous or misspelled name can land the wrong photo/cover. TDR 007
  gives a manual override (retry re-runs the same no-threshold match;
  upload replaces it with any image) but nothing surfaces confidence or
  offers a pick-from-candidates re-match UI.
- **VBRI-only MP3 duration** — the duration parser supports the Xing/Info
  VBR header (the common case); an MP3 with only a VBRI header falls back to
  the bitrate/filesize estimate, which is less accurate for that rare case.
- **Symlink handling during an import's source walk** — not specifically
  handled; `filepath.WalkDir` follows the OS's normal (non-recursive-symlink)
  traversal semantics.
- **Cancelling an in-progress import** — once confirmed, a copy runs to
  completion (or per-file failure) with no way to stop it early; TDR 005
  didn't carry over TDR 001's scan-cancellation behavior since no
  acceptance criterion asked for it.
- **Tag write-back for M4A/OGG/WAV** — TDR 005 scoped writing corrected
  tags back into the copy to MP3/FLAC only, for lack of a mature Go writer
  for the others; a copied M4A/OGG/WAV file keeps its original embedded
  tags even if the reviewer corrected them before confirming.
