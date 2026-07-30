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
rather than needing to already exist) is confined to `DATA_DIR`, which
defaults to `/data` (every compose file, root and `deploy/`, mounts the
music volume there — an operator never has to set this themselves) and is
overridable, including to unrestricted from `/`, for a plain `go run` with
no such mount to confine anything to. On top of that: a Home screen plus
Artist/Album/Song browsing
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
| Audio tag reading/duration parsing | `github.com/dhowden/tag` (ID3v2/MP4/Vorbis) + this project's own `apev2` package (WavPack's APEv2 tags, TDR 013 — dhowden/tag has no APEv2 support) + hand-rolled per-format duration parsers | `backend/internal/library/scan` (format detection + duration only) and `backend/internal/library/organize` (plan-building tag reads); see [TDR 005](tdr/005_organize_on_import_design.md) |
| Audio tag writing | `github.com/bogem/id3v2` (MP3) + `github.com/go-flac/go-flac` + `flacvorbis` (FLAC) + this project's own `apev2` package (WavPack) | `backend/internal/library/organize`; scoped to MP3/FLAC/WavPack only — no mature Go writer exists for M4A/OGG, see [TDR 005](tdr/005_organize_on_import_design.md); WavPack's APEv2 writer is hand-rolled for the same reason (TDR 013) |
| Artwork/info sourcing | MusicBrainz + Cover Art Archive + Wikidata/Wikipedia (no API key, descriptive `User-Agent`) | `backend/internal/library/enrich`; `golang.org/x/image/draw` for resizing; see [TDR 003](tdr/003_artwork_and_info_design.md) |
| Package manager (web/mobile) | pnpm workspaces, pinned via corepack (`pnpm@9`), Node.js 24 | root `pnpm-workspace.yaml` covers `web/` and `mobile/`; `Dockerfile`'s `node:24-alpine` build stage and the nightly workflow's `setup-node` both pin the same version |
| Packaging | Docker (see §1) | root `Dockerfile` + `docker-compose.yml` (local build/dev) |
| Deployment | Nightly multi-platform (amd64+arm64) image on GHCR, `.github/workflows/nightly.yml` | `deploy/docker-compose.yml` pulls it instead of building; see [TDR 004](tdr/004_self_hosted_deployment_design.md) |

## 3. Components

- **`backend/cmd/server`** — process entrypoint. Reads `PORT` (default
  `8080`), `STATIC_DIR` (set to `/app/web` in the Docker image; empty in
  local `go run`, which then serves API-only), `ARTWORK_DIR` (optional,
  like `STATIC_DIR` — unset disables embedded-art saving and the enrichment
  job entirely rather than erroring), and `DATA_DIR` (defaults to `/data`,
  confining filesystem browsing and library-root creation to under it —
  every compose file mounts the music volume there, so this doesn't need to
  be set for a Docker deployment; overridable, including to unrestricted
  from `/`, e.g. for a plain `go run` with nothing mounted at `/data` at
  all). There is no
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
  `GET /health` (liveness check, `{"status":"ok"}` only); `GET /api/about`
  (reports `{"version", "buildDate"}` — `version` is `git describe --tags
  --always` and `buildDate` a UTC timestamp, both stamped into the Docker
  image by the nightly pipeline and empty/`"dev"` for a plain `go run`;
  backs the About page, [TDR 009](tdr/009_about_page_and_versioning_design.md),
  which superseded `/health`'s original `revision` field from
  [TDR 004](tdr/004_self_hosted_deployment_design.md)); the import and
  library endpoints (below); when `ARTWORK_DIR` is set, a
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
    8MB cap) — add a manually-uploaded photo to the artist's gallery (TDR
    007; always *adds* rather than replaces as of TDR 014), bypassing
    MusicBrainz/Cover Art Archive entirely; `200` + the updated artist
    (gallery included), or `503` if `ARTWORK_DIR` isn't configured
  - `POST /api/library/artists/{id}/photos/{photoId}/primary` /
    `DELETE /api/library/artists/{id}/photos/{photoId}?deleteFile=true|false`
    (TDR 014) — mark one gallery photo as primary, or remove it (the same
    keep-vs-delete-file-on-disk choice as artist/album/library removal);
    both return the updated artist (gallery included)
  - `GET /api/library/albums/{id}` — album detail (with its track listing)
  - `DELETE /api/library/albums/{id}?deleteFiles=true|false` — remove an
    album and its tracks
  - `POST /api/library/albums/{id}/art/retry` / `POST
    /api/library/albums/{id}/art` / `POST
    /api/library/albums/{id}/covers/{coverId}/primary` / `DELETE
    /api/library/albums/{id}/covers/{coverId}?deleteFile=true|false` —
    album-flavored counterparts of the four artist routes above
  - `GET /api/library/songs/{id}/stream` — a track's audio bytes (TDR 015),
    via `http.ServeContent` against the opened file (range requests,
    206 Partial Content, and content sniffing all handled by the stdlib
    directly — no hand-rolled range parsing); `Content-Type` set explicitly
    per extension first, since Go's default `mime` package doesn't reliably
    know several of these
  - `GET /api/library/songs/{id}/playlists` — every playlist containing
    this track (TDR 028), backing the "Add to playlist" picker's
    pre-checked state
  - `GET /api/playlists` / `POST /api/playlists` `{"name"}` — list
    (sorted `recent`/`name`) / create a household-shared playlist (TDR
    028) — there's no per-user identity anywhere in this app, so, like
    every other collection, a playlist belongs to the household, not a
    person
  - `GET /api/playlists/{id}` — a playlist plus its full ordered track
    listing
  - `PATCH /api/playlists/{id}` `{"name"}` — rename, returning the fresh
    detail
  - `DELETE /api/playlists/{id}` — remove a playlist; its tracks stay in
    the library, only the playlist and its ordering are gone
  - `POST /api/playlists/{id}/tracks` `{"trackId"}` — append a track; no
    dedup, the same rule `addToQueue`'s in-memory queue already uses, so
    the same track can appear more than once
  - `DELETE /api/playlists/{id}/tracks/{playlistTrackId}` — remove one
    entry, addressed by its own `playlist_tracks` row ID rather than
    `trackId` (the only way to distinguish which occurrence of a
    duplicated track is meant), returning the fresh detail — a removed
    track can shift `coverUrls` if it was among the first four, the same
    reasoning `PATCH .../reorder` below already returns detail for
  - `PATCH /api/playlists/{id}/tracks/reorder`
    `{"playlistTrackId", "toIndex"}` — move one entry to a new position,
    returning the fresh detail; persisted server-side, unlike the
    playback queue's in-memory reorder
  - `GET /api/metadata/artists?q=` — search MusicBrainz artists by name,
    every match (not just the top-ranked one); `503` if the interactive
    search client isn't wired up (it always is, see `MusicBrainz` above)
  - `GET /api/metadata/artists/{mbid}/release-groups` — that artist's
    albums
  - `GET /api/metadata/release-groups/{mbid}/releases` — a release-group's
    specific releases/editions, with each one's track count
  - `GET /api/metadata/releases/{mbid}/tracks` — a release's full track
    listing (TDR 012, the review screen's "Look up metadata" flow)
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
  `enrich.Store` — `SetArtistArt`/`SetAlbumArt` only *add* a gallery entry
  on a `Found` write (TDR 014), never on `NotFound`/`Failed`, so a failed
  retry never touches images already in the gallery, the invariant TDR
  007's always-available retry depends on; `ResetArtistArt`/
  `ResetAlbumArt` mark status pending again without touching the gallery;
  `List/Add/SetPrimary/Delete{ArtistPhoto,AlbumCover}` (TDR 014) are the
  gallery CRUD itself — `Add*` dedupes by content hash and auto-primaries
  the first image added, `Delete*` promotes the oldest remaining image to
  primary if the deleted one was primary), `Service`
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
  Cover Art Archive entirely, *adding* it to the gallery rather than
  replacing whatever was there (TDR 014); `SetArtistPrimaryPhoto`/
  `SetAlbumPrimaryCover` and `DeleteArtistPhoto`/`DeleteAlbumCover` round
  out the gallery API, the latter reusing the existing keep-vs-delete-
  file-on-disk choice).
- **`backend/internal/library/organize`** — the organize-on-import engine
  (TDR 005): `BuildPlan` (recursive source walk, tag reads that leave a
  blank field blank rather than guessing — deliberately distinct from
  `scan.ExtractTags`'s old filename-fallback behavior; a `.wv` file's tags
  come from `apev2.Read` instead of `dhowden/tag`, and a sibling `.wvc`
  hybrid-mode correction file sets `Track.HasCorrectionFile`, TDR 013),
  `Validate` (recomputes each track's canonical
  `<Artist>/<Year>.<Album>/<NN>.<Title>` destination and on-disk conflict
  status against the plan's current, possibly reviewer-edited values — the
  server, never the client, decides what confirming would actually do; a
  `.wvc` companion's own destination is folded into the same track's
  conflict/overwrite decision rather than needing one of its own), and
  `Copy` (copies each file — and its `.wvc` companion, if any — to its
  destination, writes the plan's corrected fields back into the copy's own
  MP3/FLAC/WavPack tags, re-extracts genre and *every* embedded picture
  from the original tags for the catalog (TDR 014 — `dhowden/tag`'s
  `Picture()` only ever keeps the last one parsed, so multi-picture
  extraction bypasses it: ID3v2 `APIC` frames and FLAC `PICTURE` blocks
  via already-used dependencies exposing all of them; hand-rolled OGG
  page/packet reassembly feeding the same Vorbis-comment and FLAC-picture
  parsers a `METADATA_BLOCK_PICTURE` comment's base64 payload decodes
  into; hand-rolled MP4 box traversal for M4A's `covr` atom's `data`
  children; `apev2.ReadArtworks` for WavPack's `Cover Art (*)` items), and
  tolerates per-file failure without aborting the rest — the same
  tolerance `scan.Scanner` used for the model this replaced).
- **`backend/internal/library/scan`** — now scoped to what `organize` still
  needs: audio format detection by extension
  (mp3/flac/m4a/aac/ogg/wav/wv — `.wvc` deliberately excluded, TDR 013 AC-9)
  and per-format duration parsing. The old recursive-scan-in-place engine
  (`Scanner`, `Track`, filename-fallback tag extraction) was removed with
  TDR 005; TDR 001's original design remains in that TDR's write-up for
  history.
- **`backend/internal/library/scan/duration`** — per-format audio duration:
  exact for WAV/FLAC/MP4/WavPack (TDR 013 — read from the first block
  header's total-sample count, falling back to summing every block's count
  only for the rare file encoded without a known length upfront),
  best-effort for OGG (last page granule position) and MP3 (Xing/Info VBR
  header if present, else a bitrate/filesize estimate).
- **`backend/internal/library/apev2`** — reads and writes APEv2 tags (TDR
  013), the format WavPack (`.wv`) files use, which `dhowden/tag` doesn't
  support. `Read`/`Write` cover Artist/Album/Title/Track/Year (what the
  review screen edits) plus Genre (read-only everywhere in this app,
  matching every other format); `Write` preserves every other existing
  tag item untouched, the same overwrite-named-fields-only approach
  `organize`'s FLAC writer already takes. `ReadArtworks` (TDR 014) returns
  every distinctly-keyed `Cover Art (*)` item rather than just the front
  cover, each with its picture type parsed straight from the item's own
  key.
- **`backend/internal/library/enrich`** — the background artwork/info
  worker (TDR 003): `MusicBrainz` (search + lookup, rate-limited to its
  usage policy), `CoverArtArchive` (`FetchAll`, TDR 014 — every image
  Cover Art Archive has for a release-group's matched release, each with
  its own picture type, via the real `/release-group/{mbid}` JSON
  endpoint rather than the old single-image `/front` redirect),
  `Wikidata` (resolves a MusicBrainz "wikidata" relation into a Commons
  photo filename and an English Wikipedia article title, then fetches
  each), `ImageStore` (content-addressed: resizes into thumb/full JPEG
  variants under a hash-named subdirectory of `ARTWORK_DIR`, returning
  that hash as the dedup key TDR 014's gallery `Add*` methods check
  against; `Delete` removes a gallery entry's files and now-empty
  directory), and `Job` (orchestrates all of the above against a `Store`
  interface `*library.Store` satisfies — same "leaf package, dependency
  points one way" shape as `scan`/`library.Store.InsertTrack`).
  `Job.Run` processes every artist/album with any of art/facts/bio still
  `pending`/`failed`, not scoped to a particular scan, tracking each kind's
  outcome independently. `MusicBrainz` has a second, independent consumer
  as of TDR 012: the import review screen's on-demand "Look up metadata"
  flow (`SearchArtists`/`ArtistReleaseGroups`/`ReleaseGroupReleases`/
  `ReleaseTracks`, exposed via `library.Service.SetMusicBrainzSearch` and
  `/api/metadata/*`) searches/browses MusicBrainz interactively rather than
  by internal ID, sharing the same rate limiter but constructed
  unconditionally in `main.go` — unlike `Job`, it needs no `ARTWORK_DIR`
  (no image storage involved), so it's available even when artwork
  enrichment itself is off.
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
  copying → done), replacing the old directory-list Library page; its review
  step's per-album "Look up metadata" button opens
  `src/components/MetadataLookupModal.tsx` (TDR 012) — a four-step
  artist → album → release → track-match modal that applies everything to
  the plan in one commit, and
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
  that renders artist/album art (TDR 003). `src/components/ArtworkGallery.tsx`
  (TDR 014) is the multi-image gallery on the Artist/Album detail pages —
  every image in the entity's gallery with its source/type, a "Set
  primary" action, and Remove (reusing `RemoveModal`'s keep-vs-delete-file
  choice); an always-available "Add photo/cover" tile uploads, which
  always adds rather than replaces. `src/components/ArtActions.tsx` is now
  just the "Retry lookup" action — uploading moved into `ArtworkGallery`
  once it stopped being a single-image replace. `src/player/` (TDR 015)
  is this app's first global client state: `context.ts` defines the
  `PlayerContextValue`/`PlayableTrack` shapes, `PlayerContext.tsx`'s
  `PlayerProvider` owns the one shared `<audio>` element and is rooted in
  `AppLayout.tsx` (the layout route, so it survives every page
  navigation), `usePlayer.ts` is the consuming hook — split into three
  files so only `PlayerProvider` (a component) lives in the `.tsx` file,
  keeping fast-refresh happy. `src/components/MiniPlayer.tsx` (the docked
  bottom bar) and `QueueDrawer.tsx` (upcoming tracks, native HTML5
  drag-and-drop reorder) are rendered by `AppLayout.tsx` alongside
  `<Outlet />`; `src/components/PlayButton.tsx` is the shared per-row
  control on `SongsPage.tsx`/`AlbumDetailPage.tsx`, disabled for a
  `format === "wv"` track. `src/pages/PlaylistsPage.tsx` (TDR 028) is a
  card grid of `src/components/PlaylistCoverTile.tsx` collages (`ArtTile`'s
  sibling, not a variant — a playlist's cover is derived per-request, not
  a fetchable/retriable image); `PlaylistDetailPage.tsx` is the same
  `track-table` shape as `AlbumDetailPage.tsx` plus a drag handle (native
  HTML5 DnD, same as `QueueDrawer`) and a remove-from-playlist column,
  in-place rename, and a delete confirm. `src/components/
  AddToPlaylistMenu.tsx` is the "⋯" button every track row gets
  (`SongsPage`/`AlbumDetailPage`/`PlaylistDetailPage` itself) — rendered
  via a `createPortal` into `document.body` rather than inline, since
  `SongsPage`'s row is itself a `react-router` `<Link>` and React bubbles
  synthetic events through the component tree regardless of portals, so
  the modal still needs its own `stopPropagation` to keep a click inside
  it from reaching the row's navigation. `src/api/library.ts` is the
  typed fetch client.
- **`mobile/`** — untouched Expo starter. No navigation or API client added
  yet. *(Stale as of TDR 028 — mobile has since gained real navigation,
  an API client, offline storage, playback queue, and a playlists hub
  segment across several TDRs not yet reflected here; left as-is rather
  than backfilled in this same change, see the note this TDR's PR calls
  out.)*

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
  (cached once matched), `art_status`, `formed_year`/`country`/`genres` +
  `facts_status`, and `bio`/`bio_source_url` + `bio_status` — three
  independent `enrich_status` (`pending`/`found`/`not_found`/`failed`)
  columns, one per kind, rather than a single combined status. Its
  original single-slot `photo_thumb_path`/`photo_path` columns were
  replaced by the `artist_photos` table below (TDR 014) — `art_status`
  itself is unchanged, still driving the background job's "does this
  entity still need automatic discovery" scheduling, now independent of
  how many images actually exist.
- **`albums`** — one row per `(title, artist_id)` pair, `year`. Same
  upsert/direct-delete lifecycle as `artists` (`DELETE
  /api/library/albums/{id}`), and the same TDR 003 extension shape:
  `musicbrainz_id`, `art_status`, `label`/`country`/`genres` +
  `facts_status`, `description`/`description_source_url` +
  `description_status` — its original `cover_thumb_path`/`cover_path`
  columns replaced by the `album_covers` table below, same as `artists`.
- **`artist_photos`** / **`album_covers`** — added by
  [TDR 014](tdr/014_multiple_artworks_design.md): one row per image (not
  per entity), `FOREIGN KEY ... ON DELETE CASCADE`, `thumb_path`/
  `full_path`, `source` (`upload`/`embedded`/`enrichment`/
  `cover_art_archive`/`legacy`), `content_hash` (SHA-256 of the original
  bytes, deduped against before adding a new image), `is_primary`
  (exactly one per entity, enforced at the application layer — the one
  shown in list views/tiles), `created_at` (default-primary tiebreak:
  oldest wins). `album_covers` additionally carries `picture_type`
  (`front`/`back`/`booklet`/... from Cover Art Archive or an embedded
  tag's own picture-type byte; blank for a plain upload or a format with
  no such concept, like M4A).
- **`playlists`** / **`playlist_tracks`** — added by
  [TDR 028](tdr/028_playlists_design.md): `playlists` is `id`/`name`/
  `created_at`, household-shared like every other collection (no
  per-user identity exists anywhere in this app). `playlist_tracks` is
  the ordered join table — `playlist_id`/`track_id`
  (`FOREIGN KEY ... ON DELETE CASCADE`, so deleting a playlist or a
  track cleans up its entries) and `position` (kept contiguous from 0 by
  every write — add/remove/reorder all renumber). A playlist entry is
  addressed by its own `id`, not `track_id`, since duplicates are
  allowed (AC-6, matching the in-memory queue's own no-dedup rule) — two
  entries pointing at the same track need distinguishable IDs to be
  independently removed/reordered. A playlist's cover (AC-7) is never
  stored — it's derived per-request from its first up to 4 tracks'
  album art.

## 5. Feature-by-feature decision log

Full write-ups (alternatives evaluated, pros/cons) are in `docs/tdr/`; this is
an index with the one-line "why" for each, newest first.

| Feature | TDR | Chosen approach | Why (one line) |
|---|---|---|---|
| Playlists | [028](tdr/028_playlists_design.md) | New `playlists`/`playlist_tracks` tables, household-shared like every other collection; a playlist entry addressed by its own row ID rather than `track_id`, since duplicates are allowed; "Add to playlist" reached via long-press (mobile) or an overflow button/right-click (web) on any track row rather than a 4th per-row icon; a playlist's cover is a derived 2x2 collage of its first up to 4 tracks' album art, never stored | Grilling converged on the queue's own no-dedup rule and the icon-crowding concern already established for `AddToQueueButton` (backlog/025) — playlists are the same kind of per-track action, so they got the same answer |
| Drop the web auth gate *(reverses [022](tdr/022_token_backed_api_auth_design.md)'s app-wide enforcement)* | [024](tdr/024_drop_web_auth_gate_design.md) | No `/api/*` route is gated anymore — web and mobile share the same routes, so gating one meant gating both, and this install is LAN-only in practice. `POST/GET/DELETE /api/tokens` stay as pairing/bookkeeping only (drives the Paired Devices "last used" column via best-effort `ValidateAndTouch`, never blocks); the whole bootstrap mechanism (file + `auth_bootstrap` marker) is removed since there's nothing left to bootstrap into | GitHub issue #60: pasting a token into every fresh browser/device had become pure friction for a LAN-only install with no internet-exposure threat live; see the README's Security section for the operator's responsibility if that changes |
| In-browser audio player | [015](tdr/015_audio_player_design.md) | New `GET /api/library/songs/{id}/stream` (`http.ServeContent`, range requests handled by the stdlib); a persistent mini-player bar + queue drawer rooted in `AppLayout.tsx` (the one component that survives every route change), holding playback state in a `PlayerProvider`/`usePlayer()` React Context — this app's first global client state, no new dependency; clicking a track queues the rest of the list it came from and auto-advances; drag-and-drop queue reorder via native HTML5 DnD; `Song`/`AlbumTrack` gain a server-derived `format` field (the raw path itself never exposed) so a WavPack track's play button can be disabled, since no browser can decode that format | GitHub issue/request: play the library directly in the web app rather than it being catalog-only; grilling scoped this to local (this browser tab) playback only — no persistence across reload, no mobile, no WavPack transcoding |
| Multiple artworks per artist/album | [014](tdr/014_multiple_artworks_design.md) | Real gallery, not a single-image slot: new `artist_photos`/`album_covers` tables (one row per image, content-hash deduped, exactly one `is_primary`); manual upload always adds rather than replaces; Cover Art Archive's full typed image set fetched via `FetchAll` (front/back/booklet/...), not just the front cover; every embedded picture extracted across all five supported formats (MP3 APIC frames, FLAC PICTURE blocks, WavPack's APEv2 `Cover Art (*)` items, plus hand-rolled OGG Vorbis-comment and M4A `covr`-atom parsing, neither of which any existing dependency supports); new `ArtworkGallery` web component, mocked up and signed off before implementation | GitHub issue #14; grilling converged on full parity across every available source rather than a smaller first cut, given how much artwork this app was already silently discarding down to one image per entity |
| WavPack (.wv) support | [013](tdr/013_wavpack_support_design.md) | `.wv` recognized by format detection (was silently skipped before); new hand-rolled `scan/duration.WavPack` parser (block-header total-samples, falling back to a full block scan); new `apev2` package reads/writes APEv2 tags (Artist/Album/Title/Track/Year/Genre/cover art) since `dhowden/tag` has no APEv2 support and no mature Go library exists; a sibling `.wvc` hybrid-mode correction file is detected, copied alongside its `.wv`, and conflict-checked the same as any other file, with a small icon on its review row | GitHub issue #18; the user wanted full parity with MP3/FLAC rather than a detection-only first cut — confirmed via grilling, not assumed |
| Metadata lookup during import | [012](tdr/012_metadata_lookup_during_import_design.md) | Per-album "Look up metadata" flow on the import review screen: search MusicBrainz artists → browse their albums (release-groups) → pick a specific release (edition) → review its track listing matched onto the album's files by row order → one "Apply" commits everything; new interactive `MusicBrainz` search/browse/track-listing methods, exposed via `/api/metadata/*`, sharing the existing rate limiter but constructed independently of `ARTWORK_DIR` | GitHub issue #17; the by-ID background enrichment job (TDR 003) can't help when tags are missing/wrong going into the review step — needed a person-driven search-and-pick path, not another silent job |
| About page & build versioning | [009](tdr/009_about_page_and_versioning_design.md) | Semantic version derived from git tags (`git describe --tags --always`) plus a UTC build timestamp, stamped into the image and served from a new `GET /api/about`; new About page (last item in the top nav) displays both plus a GitHub link; `GET /health` drops its `revision` field, back to `{"status":"ok"}` only | The prior SHA-only `/health` revision had no in-app surface and no notion of a release, just an opaque hash to `curl` for |
| Home page table view | [008](tdr/008_home_page_table_view_design.md) | A shared "▦ Grid / ☰ Table" toggle above the home page's Recently added artists/albums sections; table columns (Artist/Albums/Songs, Album/Artist/Year) sourced entirely from fields the existing API responses already carry; choice remembered in `localStorage`, no backend change | GitHub issue #8; a large library benefits from scanning more rows at once with more detail per row than the card/chip grid shows, without giving up the grid for people who prefer it |
| Artwork status, manual retry & upload | [007](tdr/007_artwork_retry_and_upload_design.md) | Art status (pending/found/not_found/failed) exposed via the API and surfaced as a badge/pill wherever it's rendered; "Retry lookup" resets status to pending and wakes the background job immediately, always available even on a `found` item; "Upload photo/cover" bypasses MusicBrainz/Cover Art Archive entirely, synchronous, always available; `SetArtistArt`/`SetAlbumArt` fixed to only overwrite path columns on a `Found` write | The API previously couldn't distinguish "still looking" from "gave up," and a `failed`/`not_found` item had no way to be nudged short of a new import; scoped to Art only, Facts/Bio/Description unchanged |
| Multiple libraries | [006](tdr/006_multiple_libraries_design.md) | `LIBRARY_ROOT`/`IMPORT_SOURCE_ROOTS` env vars removed; a library (name + root folder, now creatable on the spot rather than needing to pre-exist) is created/deleted from within the app, several can exist; catalog browsing stays unified across all of them; filesystem browsing confined to `DATA_DIR` when configured (unrestricted from `/` otherwise — amended post-TDR-006, see `DATA_DIR` above); deleting a library cascades with the same keep-or-delete-files choice as artist/album removal | A fixed, deploy-time destination folder was inflexible for more than one logical collection, and coupled a purely operational choice to a redeploy rather than something changeable in the app |
| Organize-on-import *(its single-`LIBRARY_ROOT` destination superseded by [006](tdr/006_multiple_libraries_design.md))* | [005](tdr/005_organize_on_import_design.md) | Replaces add-directory/scan-in-place entirely: import copies files from a chosen source into a single `LIBRARY_ROOT`, renamed into `<Artist>/<Year>.<Album>/<NN>.<Title>`; review-before-copy with server-computed destinations/conflicts; tag write-back scoped to MP3/FLAC; direct artist/album deletion with explicit keep-or-delete-files choice | The original scan-in-place model left files wherever they started, with no consistent on-disk naming — organizing them is the point, not an optional extra step |
| Self-hosted deployment *(`/health`'s `revision` field superseded by [009](tdr/009_about_page_and_versioning_design.md))* | [004](tdr/004_self_hosted_deployment_design.md) | Nightly multi-platform image on GHCR (skip-if-unchanged, test-gated); separate `deploy/docker-compose.yml` pulling it, with bundled Postgres and multi-root music bind-mounts | Removes the Go/Node toolchain requirement from the target machine (a NAS); mirrors a proven pattern from a sibling project (docuflow) adapted for opusflow's host-mounted-library model |
| Artist/album artwork and info *(art status/manual override amended by [007](tdr/007_artwork_retry_and_upload_design.md); single-image schema superseded by [014](tdr/014_multiple_artworks_design.md))* | [003](tdr/003_artwork_and_info_design.md) | Embedded-tag art first, MusicBrainz + Cover Art Archive + Wikidata/Wikipedia fallback via a background `enrich.Job`; three independent per-kind statuses; files on disk under `ARTWORK_DIR`, not DB blobs | Free/open/no-API-key sources matching the project's anti-proprietary-protocol stance; a background job (not inline with scanning) respects MusicBrainz's rate limit and doubles as backfill for pre-existing libraries |
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
