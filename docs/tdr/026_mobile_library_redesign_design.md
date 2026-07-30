# TDR 026: Mobile Library Browsing Redesign

## 1. Context & Architectural Requirements

Mobile's Library screen today mashes together a horizontal "featured albums" strip (the first page of albums, no real curation behind that label) and a flat "recent tracks" list that dumps every track in the library via a single unpaginated `fetchCatalogTracks` call (`maxPageSize=100`, no pagination UI at all). There's no way to browse by artist, and no album detail screen — issue #69's album-tap fix (PR #74) plays an album immediately on tap rather than opening anything, since no detail screen existed at the time.

Web already solves this properly: separate `ArtistsPage`/`AlbumsPage`/`SongsPage`, each backed by the shared `useListPage` hook (search, sort, genre/year filters, real pagination via `Pager`), plus `ArtistDetailPage` (bio/facts + albums) and `AlbumDetailPage` (track table). Mobile has none of this structure — grilling confirmed the intent is to bring mobile to parity with web's browsing model, not invent a new one.

Mobile has zero navigation library today — `App.tsx` swaps screens via a single `useState<TabType>`, no stack, no back button anywhere in the app.

---

## 2. Alternatives Evaluated

### Alternative A: Three new top-level bottom-tab destinations
Grow the bottom tab bar from 4 tabs to 6 (Connect, Artists, Albums, Songs, Player, Offline), each browsing mode a persistent tab.

- **Pros**: No sub-navigation layer to build; each list is always one tap away.
- **Cons**: Six tabs is crowded for a bottom bar sized for thumb reachability on a phone; grilling confirmed a preference for keeping the bar at 4.

### Alternative B (Chosen): Library tab becomes a hub with a segmented control
"Library" stays one bottom tab; internally it shows a segmented control (Artists / Albums / Songs) switching between three real list views, each with its own detail-screen stack.

- **Pros**: Bottom bar stays uncrowded; mirrors how the current single screen is already the "browsing" destination, just properly structured underneath. Matches web's three separate list surfaces conceptually without needing three separate top-level mobile destinations.
- **Cons**: One more layer of navigation state to manage (which sub-tab, plus whatever detail stack sits on top of it) than a flat tab-per-list model.

### Alternative C: Adopt React Navigation
Bring in the standard Expo navigation library for real stack/back-gesture support and future deep-linking groundwork.

- **Pros**: The conventional, well-supported way to do multi-level navigation in an Expo app; would generalize cleanly if the app's navigation needs grow further.
- **Cons**: A new dependency and a refactor of `App.tsx`'s existing hand-rolled tab-switching to adopt it, for a need that today is just two levels deep (hub → detail → back) within one tab. Grilling confirmed hand-rolling matches this codebase's existing convention (plain `useState`-driven screen switching, same pattern `App.tsx` and `ConnectScreen`'s mode toggle already use) and avoids the dependency for a need this shallow.

---

## 3. Structural Decision

We select **Alternative B** for the tab structure and the non-library-list-adopting half of **Alternative C's rejection** (i.e., we explicitly do *not* adopt React Navigation) for navigation.

1. **Library hub**: a segmented control (Artists / Albums / Songs) renders one of three list screens. Each list screen owns infinite-scroll pagination, search, and sort/genre/year filters against the existing `/api/library/{artists,albums,songs}` endpoints (mobile's `api.ts` gains paginated fetchers alongside the existing `fetchCatalogAlbums`/`fetchCatalogTracks`, or those are extended — implementation detail, not a new backend endpoint).
2. **Navigation stack**: a small local state within the Library screen — `{ view: 'hub' } | { view: 'artist'; id: number } | { view: 'album'; id: number }` — with a back button on each detail screen returning to `'hub'`. Leaving the Library tab and returning always resets to `'hub'` (AC-6) — no persisted stack across tab switches, keeping the state genuinely simple.
3. **Artist Detail**: fetches `GET /api/library/artists/{id}` (already returns bio/facts + albums per `ArtistDetail`, same endpoint web's `ArtistDetailPage` uses) and renders it the same way web does (AC-3).
4. **Album Detail**: fetches `GET /api/library/albums/{id}` (already used by mobile's `fetchAlbumTracks` from backlog/023's album-tap fix) and renders its track table with the existing play/add-to-queue/download-track actions from `LibraryScreen`'s current per-track UI, moved here. A new "Download album" button loops `offlineStorage.downloadTrackForOffline` across every track in the album, showing a live "X of Y" count (AC-5) — no new persistence concept, just orchestration on top of the existing per-track download.
5. Tapping an album anywhere (the Albums grid, or from Artist Detail) navigates to Album Detail rather than playing immediately — reversing PR #74's tap-to-play-immediately behavior now that a real detail screen exists to navigate to instead (confirmed in grilling).

---

## 4. Cross-Workspace Implications

- **`mobile/`**: `LibraryScreen.tsx` is restructured into a hub + stack (or split into `LibraryHub.tsx` plus `ArtistsListScreen.tsx`/`AlbumsListScreen.tsx`/`SongsListScreen.tsx`/`ArtistDetailScreen.tsx`/`AlbumDetailScreen.tsx` — implementation detail). `api.ts` gains paginated artist/album fetchers and an artist-detail fetcher alongside the existing album-detail one. No new mobile dependency for this feature (draggable-flatlist is TDR 027's, not this one).
- **`backend/`**: no changes — every endpoint this needs already exists (`/api/library/artists`, `/api/library/artists/{id}`, `/api/library/albums`, `/api/library/albums/{id}`, `/api/library/songs`).
- **`web/`**: unaffected.
