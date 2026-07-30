# User Story: Add a Track to the Queue Without Interrupting Playback

## 1. User Value Statement

As a **household member browsing the library while something's already playing**,
I want **to add a single track to what's queued up next, without interrupting what's currently playing**,
So that **I can queue up more music as I go, the way I'd expect from any music app, instead of every tap replacing the whole queue**.

## 2. Strict Acceptance Criteria

- AC-1: A new `addToQueue<T>(state, track)` function in `@opusflow/player-core` appends `track` to the end of `state.queue`, leaving `currentIndex` and `isPlaying` untouched when something is already playing (`currentIndex !== -1`).
- AC-2: Calling `addToQueue` on an empty queue (`currentIndex === -1`) starts playing the added track immediately — `currentIndex` becomes `0` and `isPlaying` becomes `true` — matching what tapping it normally would do in that case.
- AC-3: Adding a track already present elsewhere in the queue appends a second copy — no deduplication.
- AC-4: Every existing track-list surface on both platforms gets a second, distinct icon per row for "add to queue," alongside the existing tap-to-play-now: mobile's `LibraryScreen`; web's `SongsPage` and `AlbumDetailPage`.
- AC-5: Tapping "add to queue" gives a brief visual confirmation (the icon itself, not a new toast/snackbar system) so the action is never a silent no-op.
- AC-6: Web's existing Queue Drawer reflects an appended track with no special-casing — it already renders `state.queue` in order.
