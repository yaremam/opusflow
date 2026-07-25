# TDR 002: Home Screen & Library Browsing

## 1. Context & Architectural Requirements

Per `docs/ARCHITECTURE.md`, at the start of this feature the web app has
exactly one screen (`LibraryPage`, rendered directly by `App`, no router) and
`tracks` stores `artist`/`album` as free-text columns with no dedicated
entities — there is no endpoint that lists tracks, artists, or albums at all,
only `GET /api/library/directories` (which returns each directory's
`trackCount`).

`docs/vision.md` describes the library as a unified home for local files and
streaming catalogs, and separately describes artist-following as a future
capability that will need to link a local artist to streaming/metadata APIs
(Spotify, Apple Music, MusicBrainz). Grilling this feature (originally scoped
as "a landing page") surfaced that a home screen only has real content to
summarize once artists/albums/songs are addressable entities in their own
right, not just free text on a track row — which pulled in a genuine
browsing surface (index + detail pages) and its underlying data model as
part of the same feature.

## 2. Alternatives Evaluated

### Alternative: Normalized `artists`/`albums` tables vs. name-based grouping

- **Name-based grouping (no new tables)** — Pros: no migration, no
  scan-time change; "artist" and "album" listings are just `GROUP BY`
  queries over the existing `tracks` table. Cons: no stable identifier to
  route Artist/Album detail pages by (would mean URL-encoding a name, with
  no way to distinguish two differently-tagged variants of what's really the
  same artist); every listing is recomputed via `GROUP BY` on every request;
  no natural place to eventually attach a streaming-service artist ID.
- **Normalized tables (chosen)** — Pros: stable numeric IDs to route detail
  pages by; a natural home for the future streaming-service linkage vision.md
  already anticipates; accurate counts without recomputing aggregates on
  every listing request; cascade-friendly cleanup when a directory is
  removed. Cons: requires a schema migration and changes the scan write path
  — each imported track now upserts an artist row and an album row before
  inserting the track, instead of one plain insert.
- **Chosen: normalized tables.** `artists(id, name)` and
  `albums(id, title, artist_id, year)`, with `tracks.artist_id` /
  `tracks.album_id` as `NOT NULL` foreign keys. Tracks with no artist/album
  tag upsert against the literal empty-string name/title (AC-11) — a real
  "Unknown Artist" / "Unknown Album" row, not a null — so every track always
  has both FKs and no query needs null-handling.

### Alternative: Index page pagination — numbered pages vs. infinite scroll

- **Infinite scroll** — Pros: no page-number UI to build; feels continuous
  for a casual scroll-through. Cons: doesn't compose well with sort/filter
  changes (where was I?); harder to reason about in tests (no fixed page
  boundary to assert against).
- **Numbered pagination (chosen)** — Pros: explicit, testable page
  boundaries; a sort or filter change has an obvious reset point (back to
  page 1); simple `page`/`pageSize` query params. Cons: one extra control to
  build (the pager) versus a scroll listener.
- **Chosen: numbered pagination.** `GET` list endpoints take `page`,
  `pageSize` (default 30), `sort` (`recent` default, or `name`), `genre`,
  `year`, and `q` (free-text, matched against name/title), and return
  `{items, page, pageSize, totalCount}`.

## 3. Structural Decision

Add `artists` and `albums` tables via a new migration; `library.Store`'s
track-insert path becomes an upsert-artist → upsert-album → insert-track
sequence (`ON CONFLICT` upserts, so concurrent scans of different
directories can't race each other into duplicate artist/album rows).
`library.Service` gains list/detail methods backing five new endpoints
(artists/albums/songs index, each paginated/sorted/filtered/searched; artist
detail; album detail). Directory removal, which already cascades track
deletion via `ON DELETE CASCADE`, gains a follow-up step in the same
transaction that deletes any artist/album row left with zero tracks
(AC-12).

The web app adds `react-router` — its first router, resolving the choice
`docs/ARCHITECTURE.md` §6 deferred "until a feature actually needs a second
page." Routes: `/` (Home), `/artists`, `/artists/:id`, `/albums`,
`/albums/:id`, `/songs`, `/library` (existing `LibraryPage`, unchanged). A
shared layout component renders the header nav (Home / Artists / Albums /
Songs / Library) around every route. Home, and the three index pages, all
call the same list endpoints (index pages just render a larger page); a song
row's link target is always its album's detail page (AC-13) — there is no
per-song detail page, since with no playback built yet there's nothing a
song page would show beyond what the album page already has.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - New migration: `artists(id, name)`, `albums(id, title, artist_id,
    year)`; `tracks` gains `artist_id`/`album_id` `NOT NULL` foreign keys.
  - `library.Store`: track insertion becomes upsert-artist/upsert-album/
    insert-track; new list/get methods for artists, albums, and tracks
    (pagination, sort, genre/year filter, text search) and their detail
    variants; directory removal gains orphan artist/album cleanup.
  - New HTTP endpoints (indicative, exact routes decided at implementation
    time): `GET /api/library/artists`, `GET /api/library/artists/{id}`,
    `GET /api/library/albums`, `GET /api/library/albums/{id}`,
    `GET /api/library/songs`.
- **`web/`**:
  - New dependency: `react-router`.
  - New shared layout component (header nav) wrapping all routes.
  - New pages: `HomePage`, `ArtistsPage`, `ArtistDetailPage`, `AlbumsPage`,
    `AlbumDetailPage`, `SongsPage`. `LibraryPage` itself is unchanged.
  - New `web/src/api/` client functions for the five new endpoints.
- **`mobile/`**: out of scope, unchanged (no networking code yet).
- **Schema**: second migration in the project, adding `artists` and
  `albums` and altering `tracks`.
- Update `docs/ARCHITECTURE.md` §3–6 once implementation lands: move the web
  router choice out of §6 into §2/§3, add `artists`/`albums` to §4, add a row
  to the §5 decision log linking back to this TDR, and update §3's `web/`
  component list with the new pages/layout.
