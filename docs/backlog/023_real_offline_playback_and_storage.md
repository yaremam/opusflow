# User Story: Real Offline Playback & Storage

## 1. User Value Statement

As a **mobile listener**,
I want **tracks to actually download, play from disk, and cache automatically while streaming — instead of the current simulated bookkeeping that never touches a real file, never survives a restart, and never actually plays any audio at all**,
So that **I can listen to my library on the go, including with no connectivity, the way the app already claims to support**.

---

## 2. Strict Acceptance Criteria

- **AC-1 (Real playback)**: The app plays actual audio via `expo-audio`, including background playback, lock-screen/notification controls, and audio-focus handling (ducking/pausing for calls) — replacing today's pure-state `audioPlayer.ts`, which never engages any audio hardware.
- **AC-2 (Real downloads)**: "Make Available Offline" downloads the real audio file (and its artwork, if any) to `FileSystem.documentDirectory` via a simple one-shot download. A failed/interrupted download can be retried by tapping again; nothing resumable is persisted.
- **AC-3 (Persistent index)**: Downloaded/cached track metadata (id, local audio path, local artwork path, size, timestamp, explicit-vs-cache) is persisted to a JSON manifest file, read back on app start — surviving an app restart, unlike today's in-memory `Map`.
- **AC-4 (Local-first playback)**: Playing a track checks the persisted index first; if a local file exists, it plays from disk with no network request. Otherwise it streams from the server as today. A network failure during that fallback surfaces as a normal error, not a crash.
- **AC-5 (Automatic LRU stream cache)**: A streamed (not explicitly downloaded) track that finishes playing is cached to disk in the background, on any network. This is invisible/automatic — no user action triggers it, matching the existing "Explicit vs. LRU Cache" distinction already in the Storage screen's meter.
- **AC-6 (Free-space-aware eviction, not a fixed quota)**: The LRU cache checks real device free space and evicts its own oldest entries first whenever free space drops below a safety margin. Explicit downloads are never auto-evicted under any circumstance — only the user's own remove action touches them. No fixed size limit (e.g. a hardcoded "2 GB") and no user-facing size-limit control.
- **AC-7 (Storage screen reflects reality)**: The existing Storage screen's usage meter, downloaded-items list, per-item remove, and "Clear Stream Cache" action all operate on the real persisted index and real files (`FileSystem.deleteAsync`) — no UI changes beyond that; today's layout/controls stay as they are.
