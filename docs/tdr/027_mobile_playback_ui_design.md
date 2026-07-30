# TDR 027: Mobile Playback UI — Queue View & Quality Indicator

## 1. Context & Architectural Requirements

`PlayerScreen.tsx`'s top-right button (`list-outline`) has never had an `onPress` handler — tapping it does nothing (issue #78). `audioPlayer.ts` only wraps `playQueue`/`togglePlayPause`/`nextTrack`/`previousTrack`/`seekTo`/`toggleShuffle`/`toggleRepeat`/`addToQueue` from `@opusflow/player-core` — it never wrapped `jumpTo`, `removeFromQueue`, or `reorderQueue`, even though the shared package has had them since backlog/019. Web's `PlayerContext` already wraps all three, backing its own Queue Drawer (drag-reorder via native HTML5 DnD, per `docs/ARCHITECTURE.md`'s TDR 015 entry) — mobile has had no equivalent.

Separately, the Player screen's "Streaming"/"Offline" badge (`qualityBadge` in `PlayerScreen.tsx`) only distinguishes source, not quality. Checked against the actual backend: `Song`/`AlbumTrack` already expose `format` (mp3/flac/m4a/ogg/wv, derived from the file extension — see `TrackFormat` in `backend/internal/library/catalog.go`), but **bitrate is not persisted or exposed anywhere** — it's a value parsed internally during MP3 duration estimation (`scan/duration/mp3.go`) and discarded immediately after computing `durationSeconds`. Displaying real quality information needs new backend work, not just a mobile-side mapping.

---

## 2. Alternatives Evaluated

### 2a. Reorder mechanism

#### Alternative A: Hand-rolled drag gesture (PanResponder/Reanimated)
Build touch-tracking, auto-scroll, and list-position math directly against React Native's gesture primitives.

- **Pros**: No new dependency.
- **Cons**: Drag-and-drop-with-auto-scroll-and-virtualization is one of the harder interactions to get right by hand on React Native — real risk of a janky or broken-feeling reorder for a feature whose entire value is feeling fluid. Grilling explicitly flagged this risk.

#### Alternative B (Chosen): `react-native-draggable-flatlist`
The established library for exactly this interaction.

- **Pros**: Real drag-and-drop feel (auto-scroll, smooth reordering, virtualization-aware) without hand-rolling gesture code. Directly wraps a `FlatList`, matching how every other list in this app is already built.
- **Cons**: One new dependency — accepted; grilling confirmed the risk of a hand-rolled version outweighs adding a well-maintained, purpose-built library.

### 2b. Bitrate

#### Alternative A: Per-format precise bitrate parsing
Parse each format's actual frame/stream headers for an exact (and VBR-aware) bitrate — MP3 already does this internally during duration estimation; FLAC/OGG/M4A would each need their own parsing logic added.

- **Pros**: Exact, format-native bitrate figures.
- **Cons**: Five formats, five different bitrate representations to parse correctly (and some, like FLAC, don't have a single "bitrate" header the way MP3 does at all) — real, open-ended parsing work for a number whose entire job is a rough "how good is this" indicator, not a precision instrument.

#### Alternative B (Chosen): Average bitrate — file size in bits ÷ duration in seconds
One formula, uniform across every format, no per-format parsing.

- **Pros**: Trivial to compute once file size and duration are both known (duration is already computed at scan time for every format) — genuinely accurate as an *average* bitrate, which is what "940 kbps" communicates to a listener regardless of whether the source is CBR or VBR. No per-format special-casing.
- **Cons**: Not exact for VBR files at any given instant (a listener sees the average across the whole track, not the true instantaneous rate) — accepted as more than sufficient for a UI indicator, matching what most consumer music software actually shows.

---

## 3. Structural Decision

1. **`audioPlayer.ts`** gains `jumpTo(index)`, `removeFromQueue(index)`, and `reorderQueue(fromIndex, toIndex)`, each a thin wrapper over `@opusflow/player-core`'s existing functions — mirroring exactly how web's `PlayerContext` already wraps them, and how `addToQueue` was wrapped in backlog/025.
2. **Queue view**: a new screen, opened from `PlayerScreen`'s top-right button, rendering `audioPlayer.getState().queue` via `react-native-draggable-flatlist`. The now-playing entry is visually distinguished and not draggable/removable from its own row (matching how the current track can't be dragged out from under itself); every other row gets a remove control and participates in reordering.
3. **Bitrate**: a new `file_size_bytes` column captured during scan/import (a simple `os.Stat`, no new parsing) is added to `tracks`. The songs API derives `bitrateKbps = fileSizeBytes * 8 / durationSeconds / 1000` at response time rather than storing a redundant derived value. `Song`/`AlbumTrack` gain this field; mobile's `Track` type and its API mappers pick it up.
4. **Player screen**: the `qualityBadge` renders `"{format.toUpperCase()} · {bitrateKbps} kbps"` instead of "Streaming"/"Offline" — the streaming-vs-offline distinction stays visible through the existing per-track download-icon convention elsewhere in the UI, it just isn't this badge's job anymore.

---

## 4. Cross-Workspace Implications

- **`backend/`**: new migration adding `tracks.file_size_bytes` (nullable — existing rows won't have it until re-scanned; the enrichment/rescan story for backfilling old libraries is out of scope here, same as any other schema addition that only benefits newly-scanned files). `library/scan` captures it during import. `Song`/`AlbumTrack` gain `fileSizeBytes`/derived `bitrateKbps` in their JSON. New unit tests for the bitrate derivation.
- **`mobile/`**: `audioPlayer.ts` (3 new wrapper methods + tests), new Queue view screen, `PlayerScreen.tsx`'s quality badge, `api.ts`'s `Track` type and mappers gain `format`/`bitrateKbps`. New dependency: `react-native-draggable-flatlist`.
- **`web/`**: unaffected — web's Queue Drawer and quality display (if any) are out of scope for this TDR, which is scoped to mobile per issue #78.
