# User Story: Mobile Library Browsing Redesign

## 1. User Value Statement

As a **household member browsing my library on the mobile app**,
I want **to browse by artist, album, and song the way I can on the web app, with real search/sort/filter and the ability to download a whole album at once**,
So that **I can actually find and manage what I'm looking for, instead of a "featured albums" strip that means nothing and a track list that dumps my entire library unfiltered**.

## 2. Strict Acceptance Criteria

- AC-1: The Library tab becomes a hub with a segmented control — **Artists / Albums / Songs** — replacing the current featured-albums strip + flat recent-tracks list. The bottom tab bar stays at 4 tabs.
- AC-2: Each of the three lists is real: infinite-scroll pagination (loads the next page as the user scrolls near the bottom), a search field, and sort/genre/year filters — matching web's Artists/Albums/Songs pages' filter set.
- AC-3: Tapping an artist opens an Artist Detail screen: bio/facts (when available) plus a grid of their albums — matching web's `ArtistDetailPage`.
- AC-4: Tapping an album (from the Albums grid or from Artist Detail) opens an Album Detail screen: cover, metadata, and a track table with per-track play / add-to-queue / download-this-track actions — matching web's `AlbumDetailPage` plus mobile's existing per-track actions from backlog/025.
- AC-5: Album Detail has a "Download album" action that downloads every track in the album via the existing per-track download path, showing a spinner and live "X of Y downloaded" count while it runs.
- AC-6: Navigation between the hub and detail screens is a back-button stack scoped to the Library tab (no navigation library dependency) — leaving the Library tab and returning resets to the hub.
