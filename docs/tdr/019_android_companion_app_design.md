# TDR 019: Android Companion App Architecture

## 1. Context & Architectural Requirements

OpusFlow currently features a Go backend (`backend/`), a React 19 + Vite web frontend (`web/`), and a scaffolded Expo React Native workspace (`mobile/`). As specified in `CLAUDE.md`, `mobile/` was chosen to share component patterns, state management, and API domain types with `web/`.

The Android Companion App provides native mobile capabilities for OpusFlow users:
- Secure connection and pairing token management.
- High-fidelity streaming audio player with background playback and lock-screen media controls.
- Explicit offline track/album downloads and automatic LRU disk stream caching.
- Shared domain audio queue state models across web and mobile.

---

## 2. Alternatives Evaluated

### Alternative A: Web View Wrapper (PWA / Trusted Web Activity)
Wrap the existing React web application (`web/`) in a Android Webview or Trusted Web Activity (TWA).

- **Pros**:
  - Zero redundant UI code.
  - Reuses web bundle directly.
- **Cons**:
  - Unreliable background audio playback on Android when system kills webview.
  - Poor lock-screen media controls and native notification integration.
  - Restricted access to native file system for offline downloads and LRU storage management.

### Alternative B (Chosen): Native Expo React Native App with Shared Core Domain State
Build a native mobile client in `mobile/` using Expo + React Native + TypeScript, leveraging native audio services (`react-native-track-player` / Expo Audio) while sharing audio queue models and state machine interfaces with `web/`.

- **Pros**:
  - Full native Android `MediaSessionCompat` foreground service integration.
  - Smooth lock-screen controls, notification actions, and audio focus ducking/pausing.
  - Direct file system control for explicit offline downloads and LRU disk caching via `expo-file-system`.
  - Shared TypeScript domain interfaces and audio state logic with `web/`.
- **Cons**:
  - Requires maintaining mobile-specific React Native UI views.

---

## 3. Structural Decision

We select **Alternative B**.

1. **State & Domain Layer**:
   - `mobile/src/store/`: Shared state pattern using Zustand / React hooks, mapping to backend `/api/catalog` and `/api/libraries` models.
2. **Security & Connection**:
   - `expo-secure-store` persists the server base URL and pairing API token.
3. **Audio Engine & Media Controls**:
   - `react-native-track-player` / Expo Audio Service handles Android Foreground Service execution, lock screen notifications, and audio focus.
4. **Offline Download & LRU Storage**:
   - `expo-file-system` downloads audio files and artwork into `FileSystem.documentDirectory/audio_cache/`.
   - SQLite (`expo-sqlite`) or key-value index maintains LRU timestamps and disk usage metrics.

---

## 4. Cross-Workspace Implications

- **`mobile/`**: Implementation of navigation stack, connection screen, library views, player controls, and offline storage manager.
- **`web/`**: Exposes mobile token generation in the Web Settings page (`web/src/pages/Settings.tsx`).
- **`backend/`**: Consumes existing Go REST endpoints (`/api/catalog`, `/api/stream/*`, `/api/libraries`). No breaking changes to existing API contracts.
