# User Story: Playlists

## 1. User Value Statement

As a **household member curating my own collections**,
I want **to create playlists, add and remove tracks from them, reorder them, and manage them from either the web app or the mobile app**,
So that **I can build "Late Night Drive" or "Sunday Coffee" once and have it everywhere, not just rely on the shared album/artist catalog**.

## 2. Strict Acceptance Criteria

- AC-1: A household-shared "Playlists" collection is available on both web (a new top-level page) and mobile (a 4th segment on the Library hub, alongside Artists/Albums/Songs) — no per-user ownership, matching every other collection in the app today.
- AC-2: A new playlist can be created with a name, either from the Playlists page/segment directly or inline from the "Add to playlist" picker (AC-5).
- AC-3: A playlist can be renamed and deleted (with a confirmation before delete, matching the existing artist/album delete confirmation pattern) from its detail screen.
- AC-4: A playlist's detail screen lists its tracks in order, each with the same play / add-to-queue actions tracks have elsewhere, plus a remove-from-playlist action and drag-to-reorder — reordering and removal persist to the backend, not just in-memory queue state.
- AC-5: Every track row (wherever one appears — Songs list, Album detail, Artist detail's albums, mobile's Library lists) gets an "Add to playlist" action via long-press (mobile) or a right-click/overflow menu (web) — not a new always-visible icon. It opens a picker listing every playlist as a checkable row (checked = already contains this track) plus an inline "+ New playlist" row that creates one and adds the track to it in the same step.
- AC-6: Adding a track already in a playlist a second time appends a second entry — no deduplication, same rule the queue's `addToQueue` already uses.
- AC-7: A playlist's tile/header shows a cover: a 2x2 collage of its first up to 4 tracks' album art, falling back to a placeholder when it has none.
- AC-8: Tapping a playlist (its tile in the grid, or a row anywhere else it might be listed) opens its detail screen — playback starts from an explicit action there, not on tap, matching how albums work today.
