# User Story: Home Page Table View

## 1. User Value Statement

As a **household member with a large library**, I want to **switch the
home page's Recently added artists/albums between a grid and a compact
table**, So that **I can scan more items at once, with more detail per row,
instead of only ever seeing a few large tiles**.

## 2. Strict Acceptance Criteria

- **AC-1**: The home page shows a "▦ Grid / ☰ Table" toggle above the
  Recently added artists/albums sections, visible whenever those sections
  render (i.e. whenever `totalSongs > 0` — same condition already gating
  them).
- **AC-2**: The toggle is shared: switching it changes both the "Recently
  added artists" and "Recently added albums" sections together, in one
  action. "Recently added songs" is unaffected either way — it stays the
  row list it already is today.
- **AC-3**: Grid view is pixel-for-pixel today's existing layout (artist
  chip row, album card grid) — unchanged.
- **AC-4**: Table view renders artists as a table with **Artist, Albums,
  Songs** columns (name alongside a small round photo/placeholder, album
  count, track count) and albums as a table with **Album, Artist, Year**
  columns (title alongside a small cover/placeholder, artist name, year) —
  every column sourced from fields already on the existing API response,
  no new endpoint or field required.
- **AC-5**: Clicking a table row navigates to that artist's/album's detail
  page — the same destination clicking its card/chip already goes to in
  grid view.
- **AC-6**: The chosen view persists across visits (stored in
  `localStorage`) — reloading or returning to the home page later keeps
  whichever view was last selected, defaulting to grid the very first time.
