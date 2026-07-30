# TDR 025: Add a Track to the Queue Without Interrupting Playback

## 1. Context & Architectural Requirements

Every track-list surface on both platforms today only has one way to play a track: `playFrom(state, tracks, startIndex)`, which replaces the entire queue with `tracks` from `startIndex` onward and starts playing immediately (backlog/019 AC-4's "click a track, queue the rest of the list it came from" behavior). There's no way to add a single track to what's already queued without interrupting current playback — confirmed via code search that this gap exists identically on both platforms, not just mobile where issue #76 was filed: web's `PlayerContext` and mobile's `audioPlayer.ts` both only wrap `playFrom`/`next`/`prev`/`jumpTo`/`removeFromQueue`/`reorderQueue` from `@opusflow/player-core` — neither has ever had an append primitive.

`@opusflow/player-core` is the shared pure queue-state-machine package (backlog/019 AC-4) both platforms' adapters sit on top of, so the new capability belongs there once, not duplicated per platform.

---

## 2. Alternatives Evaluated

### Alternative A: "Play next" (insert after the current track)
Insert the added track immediately after `currentIndex`, so it plays next regardless of what else is queued.

- **Pros**: Useful for "I want to hear this right after the current song," a real and common intent.
- **Cons**: Different semantic than the literal request ("add to queue," i.e. append) — grilling confirmed the user wants the simpler append-to-end behavior, not a priority insert. Two intents (play-next vs. add-to-end) would need two primitives and two UI affordances, doubling scope for a feature that's supposed to close a simple gap.

### Alternative B (Chosen): Append to the end of the queue
A new `addToQueue(state, track)` pushes onto `state.queue`; current playback is untouched unless the queue was empty, in which case the added track becomes current and starts playing (matching what tapping it normally would do from an empty state — an added-but-silently-not-playing track with no visible indication why would just look broken).

- **Pros**: One primitive, one UI affordance, matches the literal ask and how most "add to queue" actions behave elsewhere (Spotify/Apple Music's plain "Add to Queue," distinct from their separate "Play Next"). Composes cleanly with the existing `removeFromQueue`/`reorderQueue` — an appended track is just another queue entry, no special-casing needed anywhere downstream (including web's Queue Drawer, which already renders `state.queue` in order).
- **Cons**: No way to jump a track to the front of the queue without a separate action — out of scope here; can be a future "play next" addition on top of this same primitive if ever wanted.

### Alternative C: Context menu / long-press instead of a second icon
Tap continues to play now; a long-press (mobile) or right-click/overflow menu (web) surfaces "Add to queue" as a menu option instead of a dedicated always-visible icon.

- **Pros**: Keeps each track row visually simpler — one primary action, not two icons competing for space.
- **Cons**: Grilling confirmed a preference for a second, always-visible icon — less discoverable as a hidden gesture, and requires building a new menu component on both platforms rather than reusing each row's existing icon-button pattern (mobile's offline-download icon, web's existing per-row icon buttons).

---

## 3. Structural Decision

We select **Alternative B**, with a second per-row icon (not Alternative C's context menu) as the UI affordance.

1. **`@opusflow/player-core`**: new `addToQueue<T>(state: QueueState<T>, track: T): QueueState<T>`.
   ```ts
   export function addToQueue<T>(state: QueueState<T>, track: T): QueueState<T> {
     const queue = [...state.queue, track]
     if (state.currentIndex === -1) {
       return { ...state, queue, currentIndex: queue.length - 1, isPlaying: true }
     }
     return { ...state, queue }
   }
   ```
   Duplicates are allowed (AC-3) — no id-scan, matching the package's existing style of simple, uncomplicated array operations (`removeFromQueue`/`reorderQueue` don't dedupe or validate uniqueness either).
2. **Web**: `PlayerContext.tsx` gains an `addToQueue` callback wrapping `coreAddToQueue`, exposed alongside `playFrom`/`next`/etc. `SongsPage.tsx` and `AlbumDetailPage.tsx` each gain a second icon per track row calling it. The Queue Drawer needs no changes (AC-6).
3. **Mobile**: `audioPlayer.ts` gains a `public addToQueue(track: Track)` method. If the queue was empty, this needs the same async `loadCurrentTrack()` treatment `playQueue`/`nextTrack`/`previousTrack` already have (TDR 024's auth-header fix made these async — this reuses that same path) since a genuinely new track starts playing. If something's already playing, it's a synchronous `this.core` update + `notify()` — no native player call at all, since playback is untouched by design. `LibraryScreen.tsx`'s track row gains a second icon calling it.
4. **Confirmation (AC-5)**: each icon swaps to a checkmark for ~1.5s on tap, then reverts — no new toast/snackbar system on either platform.

---

## 4. Cross-Workspace Implications

- **`packages/player-core/`**: `src/queue.ts` gains `addToQueue`, exported from `src/index.ts`; new unit tests covering AC-1/AC-2/AC-3.
- **`web/`**: `src/player/PlayerContext.tsx` (new callback + context value), `src/pages/SongsPage.tsx`, `src/pages/AlbumDetailPage.tsx` (new per-row icon + confirmation state). No backend or schema changes.
- **`mobile/`**: `src/services/audioPlayer.ts` (new `addToQueue` method), `src/screens/LibraryScreen.tsx` (new per-row icon + confirmation state). No backend or schema changes.
- Matches CLAUDE.md's stack decisions — no new dependencies, reuses the existing shared-package pattern from backlog/019.
