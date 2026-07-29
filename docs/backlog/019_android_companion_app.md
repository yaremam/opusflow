# User Story: Android Companion App

## 1. User Value Statement

As a **mobile music listener**,  
I want an **Android companion application for OpusFlow that streams audio, browses library metadata, manages background playback, and caches tracks offline**,  
So that **I can listen to my personal self-hosted music collection on my phone anywhere with full audio controls and lock-screen media integration**.

---

## 2. Strict Acceptance Criteria

- **AC-1 (Server Connection & Pairing)**: The app shall allow entering a server host URL and pairing token, storing credentials securely in Android `expo-secure-store`.
- **AC-2 (Library Browsing & Search)**: The app shall fetch and display artists, albums, tracks, and playlists from the backend `/api/catalog` endpoints with artwork rendering and instant search filtering.
- **AC-3 (Background Audio & Media Controls)**: The app shall play audio streams via native `MediaSessionCompat` foreground service, enabling play/pause/skip/seek from lock-screen notifications and device volume controls, while handling audio focus ducking/pausing during incoming calls.
- **AC-4 (Shared Core Audio State)**: Audio queue management, current track state, and repeat/shuffle modes shall use a shared domain state machine aligned across `mobile/` and `web/`.
- **AC-5 (Explicit Offline Downloads)**: The app shall provide a "Make Available Offline" button on tracks, albums, and playlists, storing encrypted/raw audio files in `FileSystem.documentDirectory` and updating track availability status.
- **AC-6 (Smart LRU Stream Cache)**: Streamed tracks shall automatically be cached in an LRU disk cache up to a user-configured storage limit (default 2 GB), with old entries purged automatically when capacity is reached.
- **AC-7 (Storage Management)**: The app shall feature an Offline Storage screen displaying current disk usage broken down by explicit downloads vs LRU stream cache, with options to purge cache entries.
