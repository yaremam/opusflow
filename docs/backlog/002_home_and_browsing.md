# User Story: Home Screen & Library Browsing

## 1. User Value Statement

As a **household member**, I want to **land on a home screen that summarizes
my library and lets me browse it by artist, album, and song**, So that **I
can quickly see what's in my library and find music without digging through
raw directories**.

## 2. Strict Acceptance Criteria

- **AC-1**: Visiting `/` shows a Home screen with a greeting and three live
  summary stats: total artist count, total album count, total song
  (track) count.
- **AC-2**: If any registered directory is currently scanning, Home shows a
  banner naming the directory and its progress (files processed / total).
- **AC-3**: Home shows up to 8 recently-added artists, up to 8 recently-added
  albums, and up to 8 recently-added songs. Each artist links to its Artist
  detail page, each album to its Album detail page, and each song to its
  album's detail page.
- **AC-4**: If the library has zero songs, Home shows a welcome empty state
  instead of stats/previews, with a call-to-action linking to the Library
  page.
- **AC-5**: A persistent header nav (Home / Artists / Albums / Songs /
  Library) is present on every page and highlights the current page.
- **AC-6**: An Artists index page (`/artists`) lists every artist, paginated,
  default-sorted by most-recently-added, with options to sort alphabetically
  by name, filter by genre, filter by year, and free-text search by name.
- **AC-7**: An Albums index page (`/albums`) lists every album with the same
  sort/filter/search/pagination capabilities as Artists (genre/year derived
  from the album's tracks).
- **AC-8**: A Songs index page (`/songs`) lists every track with the same
  sort/filter/search/pagination capabilities.
- **AC-9**: An Artist detail page (`/artists/:id`) shows the artist's name
  and every album attributed to them, each linking to its Album detail page.
- **AC-10**: An Album detail page (`/albums/:id`) shows the album's title,
  year, linked artist (linking to the Artist detail page), and its full
  track listing (track number, title, duration) ordered by track number.
- **AC-11**: Every imported track is attributed to exactly one artist and one
  album at scan time. Tracks with no artist/album tag are attributed to a
  real "Unknown Artist" / "Unknown Album" entity rather than excluded from
  browsing.
- **AC-12**: When a directory is removed, any artist or album left with zero
  remaining tracks as a result is deleted along with it.
- **AC-13**: Clicking a song row (on Home or the Songs index) navigates to
  that song's Album detail page.
