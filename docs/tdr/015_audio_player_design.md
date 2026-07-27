# TDR 015: In-Browser Audio Player

## 1. Context & Architectural Requirements

opusflow can import, organize, and browse a music library, but has no way
to actually listen to it from the web app — audio files are copied to
disk and cataloged, never served over HTTP. Grilling this converged on a
real listening experience for a first pass, not a minimal preview: a
persistent mini-player (survives page navigation), a queue that
auto-advances through the list a track was played from, full transport
controls (play/pause/seek/skip/volume), and a reorderable queue view —
scoped to `web/` only, in-memory playback state only (no persistence
across reload), and explicitly not attempting to solve WavPack playback
(no browser can decode it; transcoding is out of scope).

Research into the current system found:

- **No audio is ever served.** The only static file route today is
  `/artwork/` (`httpserver.go`, wired to `ARTWORK_DIR`). Track rows
  (`library.Song`, `library.AlbumTrack`) don't even carry the file
  extension in their API response, let alone expose the file itself —
  `tracks.path` (the absolute on-disk path) is read by the backend
  (`InsertTrack`, deletion's file-removal sweep) but never returned to
  the client.
- **`ListSongs`** (`catalog.go`) selects id/title/artist/album/cover
  thumb/track_number/year/genre/duration_seconds/created_at — no path,
  no format. **`GetAlbum`**'s track query (also `catalog.go`) selects
  even less: id/title/track_number/duration_seconds.
- **No global client state exists.** Every page in `web/src/pages/`
  manages its own `useState`/fetch independently; there's no Context
  provider, no state library (Redux/Zustand/etc.) anywhere in `web/`.
  `AppLayout.tsx` (`web/src/components/`) is the one component that
  never unmounts across route changes — it renders the persistent header
  nav and an `<Outlet />` for the routed page — making it the natural,
  and only existing, place to root anything that must survive
  navigation between pages.
- **Every list/detail page that shows tracks** already fetches enough to
  render a play button next to each row: `SongsPage.tsx` (via
  `listSongs`) and `AlbumDetailPage.tsx` (via `getAlbum`, whose
  `AlbumDetail.tracks` is exactly an album's track listing in play
  order).

## 2. Alternatives Evaluated

### Alternative: backend range-request handling — hand-rolled vs. `http.ServeContent`

- **Hand-rolled `Range` header parsing** — Pros: full control over
  headers/behavior. Cons: HTTP range semantics (single vs. multi-range,
  `If-Range`, `416 Range Not Satisfiable`, `Last-Modified`/`ETag`
  interplay) are notoriously easy to get subtly wrong, and this project
  has no existing need to hand-roll something Go's stdlib already solves
  correctly.
- **`http.ServeContent` (chosen)** — Go's stdlib already implements
  correct byte-range serving (206 Partial Content, conditional requests,
  content sniffing) against any `io.ReadSeeker` — an opened `*os.File`
  qualifies directly. Pros: a few lines, battle-tested, exactly satisfies
  AC-7 with no custom range-parsing code to maintain. Cons: none material
  — this is the standard idiomatic Go answer to "serve a seekable file
  over HTTP."

### Alternative: player state — React Context vs. a state management library

- **A state library (Zustand/Redux/etc.)** — Pros: more tooling for
  complex state (devtools, middleware). Cons: this project has never
  needed one — every page's state is local `useState`; introducing one
  now, for what's fundamentally "one track, one queue, one playback
  position," would be new architectural weight the app doesn't otherwise
  carry.
- **React Context + a custom hook (chosen)** — A `PlayerProvider`
  rooted in `AppLayout.tsx` (the one component that survives every route
  change) holds the queue/current-track/playback state; a `usePlayer()`
  hook exposes it plus actions (`playFrom`, `pause`, `resume`, `seek`,
  `next`, `prev`, `removeFromQueue`, `reorderQueue`) to any component.
  Matches this codebase's existing "plain React, custom hooks, no extra
  dependency" pattern (e.g. `useListPage.ts`).

### Alternative: identifying un-playable (WavPack) tracks — expose format vs. infer client-side vs. attempt-and-fail

- **Attempt playback, handle the browser's decode error** — Pros: no API
  change. Cons: the play button would need to flip into a "failed" state
  after the fact rather than never offering a false affordance — worse
  UX than simply not enabling it, and explicitly rejected during
  grilling.
- **Infer format from the title/other existing fields client-side** —
  Not viable at all: nothing in the current `Song`/`AlbumTrack` response
  carries the file extension or any derived format hint.
- **Expose a computed `format` field (chosen)** — `Song`/`AlbumTrack`
  gain a `format` string (the file extension, lowercased, with the dot
  stripped: `"mp3"`/`"flac"`/`"m4a"`/`"ogg"`/`"wv"`), derived server-side
  from `tracks.path` — the raw path itself is never sent to the client,
  same privacy stance as artwork's relative `/artwork/...` URLs never
  exposing `ARTWORK_DIR`'s real filesystem location.

### Alternative: one shared `<audio>` element vs. one per track row

- **One `<audio>` element per row** — Pros: none identified. Cons:
  multiple concurrent decode contexts, no natural single source of
  truth for "what's playing right now," and doesn't match the
  one-thing-plays-at-a-time model every real player (and this app's
  mini-player bar) assumes.
- **One shared `<audio>` element, owned by `PlayerProvider` (chosen)** —
  its `src` is set to the current track's stream URL; `usePlayer()`'s
  actions imperatively call `.play()`/`.pause()`/set `.currentTime` on a
  ref to it. The `timeupdate`/`ended` events feed the progress bar and
  drive auto-advance.

## 3. Structural Decision

### Backend

- New sentinel `ErrSongNotFound` (mirrors `ErrArtistNotFound`/
  `ErrAlbumNotFound`).
- New `Store` method resolving a track ID to its on-disk path (a `path`
  column read, nothing else) — used only by the streaming handler, never
  serialized into any list/detail JSON response.
- `ListSongs`/`GetAlbum`'s track query both add `t.path`/`path` to their
  `SELECT`, deriving a `Format` field (`filepath.Ext`, lowercased, dot
  stripped) added to `Song` and `AlbumTrack`. The path itself is
  discarded after deriving `Format` — never part of either JSON shape.
- New `GET /api/library/songs/{id}/stream` — resolves the ID to its path
  (`404` via `ErrSongNotFound` if missing), opens the file, and calls
  `http.ServeContent(w, r, filepath.Base(path), zeroTime, file)`. No new
  migration — `tracks.path` already exists; this is a read-and-serve
  addition, not a schema change.
- Content-Type is set explicitly per extension before calling
  `ServeContent` (`audio/mpeg`, `audio/flac`, `audio/mp4`, `audio/ogg`,
  `audio/x-wavpack`) rather than relying on Go's default `mime` package,
  which doesn't reliably know several of these.

### Frontend

- `PlayerProvider` (new `web/src/player/PlayerContext.tsx`) wraps
  `AppLayout.tsx`'s existing content, holding: `queue: Track[]`,
  `currentIndex`, `isPlaying`, `currentTime`, `duration`, `volume`. Owns
  the one shared `<audio>` element (rendered inside the provider, not
  visible). Exposes `usePlayer()`: `playFrom(list, startIndex)` (used by
  both entry points — sets the queue to `list.slice(startIndex)` and
  starts playing), `pause`/`resume`, `seek(time)`, `next`/`prev`,
  `removeFromQueue(index)`, `reorderQueue(from, to)`.
- New `web/src/components/MiniPlayer.tsx` (the docked bottom bar:
  art/title/artist, transport controls, seek bar, volume, a "Queue"
  toggle) and `web/src/components/QueueDrawer.tsx` (the panel: upcoming
  tracks, native HTML5 drag-and-drop reorder — no new dependency, this
  app has no existing drag-and-drop library and the interaction is
  simple enough for the platform primitives — remove, click-to-jump).
  Both rendered by `AppLayout.tsx` alongside the existing `<Outlet />`,
  so they persist across every route change exactly like the header nav
  does.
- `AlbumDetailPage.tsx`'s track table and `SongsPage.tsx`'s rows each get
  a play button per row, calling `playFrom` with that page's current
  track list and the clicked row's index. A track whose `format ===
  "wv"` renders its play button `disabled`, with a title/tooltip
  explaining why.
- `src/api/library.ts` gains a `streamURL(id)` helper building
  `/api/library/songs/${id}/stream` — no fetch wrapper needed, it's
  handed straight to the `<audio>` element's `src`.

## 4. Cross-Workspace Implications

- **`backend/`**: `library.Song`/`AlbumTrack` gain a `Format` field;
  `catalog.go`'s `ListSongs`/`GetAlbum` queries read `path` (never
  serialized) to derive it; new `Store` method resolving a song ID to
  its path; new `ErrSongNotFound` sentinel; new
  `GET /api/library/songs/{id}/stream` handler in `httpserver/`. No
  migration — no schema change.
- **`web/`**: new `PlayerProvider`/`usePlayer()` rooted in
  `AppLayout.tsx`; new `MiniPlayer.tsx`/`QueueDrawer.tsx` components;
  play buttons added to `AlbumDetailPage.tsx` and `SongsPage.tsx`; new
  `streamURL` helper in `src/api/library.ts`. Mocked up (Artifact) and
  signed off before any component code was written, per this repo's
  process for new UI screens.
- **`mobile/`**: out of scope, unchanged.
- Update `docs/ARCHITECTURE.md`'s module/API sections once this ships.
