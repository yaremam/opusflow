# TDR 023: Real Offline Playback & Storage

## 1. Context & Architectural Requirements

TDR 019 (Android Companion App) specified explicit offline downloads, an automatic LRU stream cache, and background playback with lock-screen controls. What actually shipped was UI wiring around a fully simulated `offlineStorage.ts`: no real file ever gets downloaded (sizes are guessed from `durationSeconds * 128000`, not measured), nothing persists past an in-memory `Map` (wiped on every restart), `cacheStreamedTrack` is never called from real playback, and — more fundamentally — `mobile/src/services/audioPlayer.ts` never engaged any actual audio hardware at all; no `expo-audio`, `expo-av`, or `react-native-track-player` was ever installed. "Offline playback" presupposes playback; neither existed for real.

This TDR covers wiring up both together, since "play from a local file" only means something once something actually plays.

---

## 2. Alternatives Evaluated

### Alternative A: Audio engine — `react-native-track-player`
TDR 019's other named option; purpose-built for music-app queue/media-session integration.

- **Pros**: Mature, widely used specifically for this shape of app (persistent queue, lock-screen transport controls).
- **Cons**: A heavier third-party native module with its own linking/config surface, on top of Expo's config-plugin model. As of Expo SDK 57, `expo-audio` covers the same ground (background playback via a config plugin, `setActiveForLockScreen` for lock-screen/notification controls, `interruptionMode` for audio-focus ducking) natively — adding a second, heavier audio stack for capabilities the SDK already provides isn't justified.

### Alternative B (Chosen): Audio engine — `expo-audio`
Native SDK 57 module already in the same family as `expo-camera`/`expo-secure-store`, already in this codebase.

- **Pros**: No new native-linking surface beyond a config-plugin entry (matching `expo-camera`'s pattern from TDR 022). Background playback, lock-screen controls, and interruption/ducking handling all natively supported.
- **Cons**: None material for this app's scope (a personal household player, not a competing-with-Spotify feature set).

### Alternative C: Persistence — `expo-sqlite`
TDR 019's other named persistence option.

- **Pros**: Real queries (sort by last-played, sum bytes) instead of JS-side filtering.
- **Cons**: The dataset is a phone's own local cache — realistically hundreds of rows, not something that benefits from a query engine. A JSON manifest read into memory on startup does everything the existing (already-working) in-memory bookkeeping in `offlineStorage.ts` does today, just persisted — no new dependency, no schema/migration story to maintain on-device.

### Alternative D (Chosen): Persistence — JSON manifest file
One file under `FileSystem.documentDirectory`, read on startup, rewritten on every mutation.

- **Pros**: Zero new dependencies; the existing `OfflineStorageManager` class's logic (filter explicit vs. LRU, sort by timestamp, sum sizes) carries over almost unchanged — it just also serializes to disk.
- **Cons**: A rewrite-whole-file-on-every-change strategy doesn't scale to a huge item count — acceptable here; this isn't a general-purpose database.

### Alternative E: Fixed storage quota (e.g. hardcoded 2 GB, or a user-configurable limit)
TDR 019's original AC-6 language ("up to a user-configured storage limit, default 2 GB").

- **Pros**: Predictable, simple to reason about; matches the literal original acceptance criterion.
- **Cons**: An arbitrary number that doesn't account for how much storage a given phone actually has (2 GB is generous on a 32 GB phone, meaningless on a 512 GB one) or how much a user actually wants to dedicate — and needing a settings control to change it is new UI surface for a number nobody has good intuition for. Superseded by Alternative F.

### Alternative F (Chosen): Free-space-aware LRU eviction
Check real device free space (`expo-file-system`); evict the LRU cache's own oldest entries once free space drops below a fixed safety margin. No user-facing limit at all.

- **Pros**: Scales correctly across any device automatically; no settings UI needed. Explicit downloads are structurally exempt from this eviction (only LRU-flagged entries are ever candidates), so a user's deliberate downloads are never silently removed to make room for automatic caching.
- **Cons**: Less "predictable" in the abstract (a user can't see a number and know exactly how much is reserved for opusflow) — acceptable given the Storage screen's meter already shows current usage transparently.

---

## 3. Structural Decision

We select **Alternative B + D + F**.

1. **Playback (`mobile/src/services/audioPlayer.ts`)**: rebuilt on `expo-audio`. The existing `@opusflow/player-core` queue state machine (TDR 023 doesn't touch this — it's already correct and shared with web) stays; this TDR replaces only the leaf adapter that turns "here's the current track" into actual sound, background session, and lock-screen metadata.
2. **Downloads (`mobile/src/services/offlineStorage.ts`)**: `FileSystem.downloadAsync` for both the audio file and artwork (if present) into a dedicated app directory. One-shot; a failed download is surfaced as an error and requires the user to tap again — no resumable-download state.
3. **Persistence**: a single JSON manifest file under `FileSystem.documentDirectory`, holding exactly what `OfflineStorageManager`'s in-memory `Map` holds today (id, title, artist, album, local audio path, local artwork path, size, timestamp, explicit-vs-LRU flag), read into memory on construction and rewritten after every mutation.
4. **Local-first playback**: before asking `audioPlayer` to play a queue, each track's local availability is checked against the manifest; a locally-available track's local `file://` path is substituted for its network `streamUrl`. No connectivity library — a network fallback that fails surfaces as a normal playback error.
5. **Automatic LRU caching**: `cacheStreamedTrack` is now actually invoked — wired into `audioPlayer`'s "track finished" event, not track start, so a skipped track is never cached. Runs regardless of network type (explicit downloads are the user's own bandwidth choice; auto-caching finishing a stream that already happened doesn't cost anything extra the user didn't already spend).
6. **Eviction**: before writing a newly-finished LRU cache entry, check `expo-file-system`'s free-space report; if below a fixed safety margin (1 GB), delete the oldest LRU-flagged entries (oldest `downloadedAt` first) — real files via `FileSystem.deleteAsync`, not just manifest bookkeeping — until back above the margin. Explicit entries are never candidates.
7. **Storage screen**: no layout/control changes (AC-7) — its existing meter, list, per-item remove, and "Clear Stream Cache" button now read/act on the real manifest and real files instead of the in-memory mock.

---

## 4. Cross-Workspace Implications

- **`mobile/` only.** No backend or web changes — the backend's existing `/api/library/songs/{id}/stream` and artwork URLs are consumed exactly as they are today; this TDR is entirely about what the mobile client does with them once fetched.
- New dependencies: `expo-audio` (replacing the no-op playback layer), no new dependency for persistence (JSON via `expo-file-system`, already installed) or eviction (same `expo-file-system` free-space APIs).
- `app.json` gains an `expo-audio` config-plugin entry (background playback), same pattern as TDR 022's `expo-camera` entry.
- Supersedes TDR 019 AC-6's "user-configured storage limit (default 2 GB)" language with free-space-aware eviction (Alternative F) — no settings UI is added as a result; `docs/ARCHITECTURE.md`'s mobile section should be updated once this ships to describe the real (not simulated) storage/playback behavior.
