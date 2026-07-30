# TDR 028: Playlists

## 1. Context & Architectural Requirements

opusflow has no playlist concept at all today — the only mention anywhere in the codebase is an aspirational line in TDR 019's backlog entry that was never built. The catalog is entirely artist/album/track-derived; there's no user-created, arbitrarily-ordered collection of tracks anywhere.

`docs/vision.md`'s household model gestures at per-profile playlists eventually ("each profile has its own listening history, follows, recommendations"), but no profile/identity system exists yet — every current collection (libraries, paired devices) is household-shared, not per-person. Playlists follow the same shape for now (confirmed in grilling): shared, not owned by anyone specific.

`@opusflow/player-core` already has `addToQueue`/`removeFromQueue`/`reorderQueue` (backlog/025, backlog/019) for the in-memory playback queue — playlists need the same three operations, but persisted to Postgres instead of held in memory, since a playlist survives across sessions and devices.

---

## 2. Alternatives Evaluated

### Alternative A: A 4th always-visible icon per track row for "add to playlist"
Matches how play/add-to-queue are already surfaced.

- **Pros**: Maximally discoverable, consistent with the existing icon pattern.
- **Cons**: Track rows already carry 2 icons on web (play, add-to-queue) and 3 on mobile (play via tap, add-to-queue, download) — a 4th risks real crowding, especially on mobile's narrower rows. Grilling confirmed a preference for keeping rows as they are.

### Alternative B (Chosen): Long-press / overflow-menu context action
Tap still plays; a long-press (mobile) or right-click/overflow "⋯" (web) opens a menu with "Add to playlist" (and room for future per-track actions without further crowding the row itself).

- **Pros**: Rows stay exactly as they are today; a menu scales to more actions later (e.g. "Go to album") without ever needing a 5th, 6th icon. Confirmed in grilling.
- **Cons**: Less discoverable than an always-visible icon — accepted as the right tradeoff for an action used less often than play/queue.

### Alternative C: Playlist track removal/reorder addressed by track ID
`DELETE /api/playlists/{id}/tracks/{trackId}` — removes by track ID.

- **Pros**: Simpler URL shape, one fewer concept (no need to expose a join-table row ID).
- **Cons**: Breaks the moment a playlist contains the same track twice (AC-6, explicitly allowed, matching the queue's own no-dedup rule) — there'd be no way to address *which* occurrence to remove or reorder. Rejected in favor of Alternative D.

### Alternative D (Chosen): Addressed by the `playlist_tracks` join-row ID
Each entry in a playlist has its own stable ID (`playlistTrackId`), independent of which track it points at.

- **Pros**: Removal and reordering are unambiguous even with duplicate tracks in the same playlist. Matches how `removeFromQueue`/`reorderQueue` already work by *index* into the queue array, not by track identity — same principle, just a stable ID instead of a position that shifts.
- **Cons**: One more ID for clients to track alongside `trackId` — acceptable, it's already returned on every playlist-track row.

---

## 3. Structural Decision

1. **Schema**: two new tables.
   ```sql
   CREATE TABLE playlists (
       id         BIGSERIAL PRIMARY KEY,
       name       TEXT NOT NULL,
       created_at TIMESTAMPTZ NOT NULL DEFAULT now()
   );

   CREATE TABLE playlist_tracks (
       id          BIGSERIAL PRIMARY KEY,
       playlist_id BIGINT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
       track_id    BIGINT NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
       position    INT NOT NULL,
       added_at    TIMESTAMPTZ NOT NULL DEFAULT now()
   );
   CREATE INDEX playlist_tracks_playlist_id_position_idx ON playlist_tracks(playlist_id, position);
   ```
   `track_id` cascades so deleting a track from the library (directly, or via its artist/album) removes it from every playlist automatically — no orphaned rows, no separate cleanup job, mirroring how every other FK in this schema already behaves.

2. **API** (`backend/internal/library`, new `playlists.go`, following `libraries.go`'s plain-CRUD shape rather than `catalog.go`'s enrichment-heavy one — playlists have no MusicBrainz/background-job concept):
   - `POST /api/playlists` — `{name}` → `Playlist`
   - `GET /api/playlists` — paginated (`page`/`pageSize`/`sort` — "recent" or "name"; no genre/year, playlists don't have them) → `Page[Playlist]`
   - `GET /api/playlists/{id}` — `PlaylistDetail` (ordered tracks)
   - `PATCH /api/playlists/{id}` — `{name}`, rename (AC-3)
   - `DELETE /api/playlists/{id}` — delete (AC-3); cascades to its `playlist_tracks` rows, never touches the underlying tracks/files
   - `POST /api/playlists/{id}/tracks` — `{trackId}` → appends at the end (AC-6: no dedup, mirrors `addToQueue`)
   - `DELETE /api/playlists/{id}/tracks/{playlistTrackId}` — removes one entry (Alternative D)
   - `PATCH /api/playlists/{id}/tracks/reorder` — `{playlistTrackId, toIndex}` → persists a reorder, recomputing every affected row's `position`
   - `GET /api/library/songs/{id}/playlists` — every playlist containing this track (by `trackId`), used to pre-check the "Add to playlist" picker (AC-5)

3. **`Playlist`**:
   ```go
   type Playlist struct {
       ID         int64     `json:"id"`
       Name       string    `json:"name"`
       TrackCount int       `json:"trackCount"`
       CreatedAt  time.Time `json:"createdAt"`
       CoverURLs  []string  `json:"coverUrls"` // up to 4 thumbs, oldest-position-first; empty when the playlist has no tracks
   }
   type PlaylistDetail struct {
       Playlist
       Tracks []PlaylistTrack `json:"tracks"`
   }
   type PlaylistTrack struct {
       PlaylistTrackID    int64  `json:"playlistTrackId"`
       TrackID            int64  `json:"trackId"`
       Title              string `json:"title"`
       ArtistName         string `json:"artistName"`
       AlbumTitle         string `json:"albumTitle"`
       AlbumCoverThumbURL string `json:"albumCoverThumbUrl"`
       DurationSeconds    int    `json:"durationSeconds"`
       Format             string `json:"format"`
   }
   ```
   `CoverURLs` (AC-7) is the first up to 4 tracks' `album_covers` primary thumbnail, ordered by `playlist_tracks.position` — computed at read time via a `LATERAL` join limited to 4 rows, the same join-and-limit shape `artistPrimaryPhotoJoin`/`albumPrimaryCoverJoin` already use for a single image.

4. **Web**: new `PlaylistsPage.tsx` (same `card-grid` pattern as `AlbumsPage.tsx`) and `PlaylistDetailPage.tsx` (same track-table shape as `AlbumDetailPage.tsx`, plus a drag handle and remove control per row, calling the reorder/remove endpoints directly — not `@opusflow/player-core`'s in-memory `reorderQueue`, since this is persisted state, not playback state). `PlayButton`/`AddToQueueButton`'s sibling: an overflow "⋯" button opening a small menu with "Add to playlist," reusing the same picker component from both entry points (Songs, Album detail, Artist detail, and the new Playlist detail itself).

5. **Mobile**: `LibraryScreen.tsx`'s segmented control gains a 4th segment, `PlaylistsListScreen.tsx` (mirrors `AlbumsListScreen.tsx`), `PlaylistDetailScreen.tsx` (mirrors `AlbumDetailScreen.tsx`, reusing `react-native-draggable-flatlist` — already a dependency since backlog/027's Queue view — for reorder). Track rows across `SongsListScreen`/`AlbumDetailScreen`/`ArtistDetailScreen`'s albums gain `onLongPress` opening the same picker (a new bottom-sheet component), rather than a new icon (Alternative B).

---

## 4. Cross-Workspace Implications

- **`backend/`**: new migration (`playlists`, `playlist_tracks`), new `internal/library/playlists.go` (store methods) + `internal/httpserver/playlists.go` (handlers), new `library.Service` methods delegating to the store (matching `libraries.go`'s pattern), route registration in `httpserver.go`. `catalog.go` gains the one new `GET .../playlists` membership-lookup query.
- **`web/`**: new `PlaylistsPage.tsx`, `PlaylistDetailPage.tsx`, `AddToPlaylistMenu.tsx` (or similar — the overflow button + picker), new `api/library.ts` functions. Router gains `/playlists` and `/playlists/:id`.
- **`mobile/`**: new `PlaylistsListScreen.tsx`, `PlaylistDetailScreen.tsx`, an `AddToPlaylistSheet.tsx` component, `api.ts` gains the matching fetchers, `LibraryScreen.tsx`'s stack/segmented-control state extends to the 4th segment and a `playlist` detail view.
- No changes to `@opusflow/player-core` — playlist ordering is persisted backend state addressed by `playlistTrackId`, deliberately not routed through the in-memory queue reducer that already owns a different concept (what's currently playing).
