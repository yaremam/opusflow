# TDR 008: Home Page Table View

## 1. Context & Architectural Requirements

GitHub issue #8 asked for a "table view for albums and artists in the home
page." The home page (`web/src/pages/HomePage.tsx`, TDR 002) currently
renders "Recently added artists" as a chip row and "Recently added albums"
as a card grid, each pulling the same `Artist`/`Album` list data the
`/artists` and `/albums` index pages use — no new backend data is needed;
every field a table would show (album/track counts, artist name, year) is
already on the response. This is a purely frontend, purely presentational
change: a second layout for data already being fetched, plus a toggle to
pick between them.

The mockup (grilled and signed off before this doc) established the shape:
one shared toggle above both sections, table columns drawn straight from
existing fields, row clicks navigating exactly where a card/chip already
does, and the choice remembered across visits.

## 2. Alternatives Evaluated

### Alternative: remembering the chosen view — localStorage vs URL query param vs no persistence

- **URL query param** (`?view=table`) — Pros: shareable/bookmarkable,
  visible in the address bar. Cons: the home page has no other query-param
  state today, and a view preference isn't really a "this specific page
  state" the way a filter is — it's a standing preference, more like a
  setting than a navigation parameter.
- **No persistence (component state only)** — Pros: simplest possible
  implementation. Cons: directly contradicts AC-6 — resets to grid on
  every visit, which is exactly what a household member picking table view
  because they have a large library would find most annoying.
- **`localStorage` (chosen)** — Pros: persists across visits/reloads with
  no backend involvement — there's no user-accounts/preferences system in
  this app to store it server-side anyway, and one boolean doesn't warrant
  starting one. Cons: local to one browser/device — a different browser
  starts back at grid, accepted as fine for a single-household, mostly
  single-device-per-person app.

### Alternative: toggle grouping — one shared toggle vs a toggle per section

- **Toggle per section** — Pros: maximum flexibility (table artists,
  grid albums, say). Cons: two controls doing conceptually the same job
  clutters the page for a distinction most people won't want; the signed-off
  mockup shows one control, not two.
- **One shared toggle (chosen)** — Pros: a single, obvious control; matches
  the signed-off mockup. Cons: can't mix views between the two sections —
  acceptable, nobody asked for that and it's not what the issue described.

## 3. Structural Decision

A `viewMode` state (`'grid' | 'table'`) in `HomePage.tsx`, initialized from
a `localStorage` key and written back to it on every change — no new
component needed for the persistence itself, just a small
`useState`+`useEffect` pair, consistent with how little other client-side
state this app persists today (nothing does yet; this is the first).

Rendering: the existing chip-row/card-grid JSX is kept as the `grid` branch,
untouched (AC-3). A new `table` branch renders two `<table>` elements (one
per section) using the same `data.artists`/`data.albums` arrays already
being fetched — no new API call, no new fields. Each row is a full-row
click target (`onClick` navigating via `useNavigate`, consistent with how
`ArtTile`'s callers already wrap cards in a `<Link>` today) rather than a
`<Link>`-per-cell, since a table row reads more naturally as one clickable
unit than a card does.

## 4. Cross-Workspace Implications

- **`backend/`**: untouched — no new fields, no new endpoint.
- **`web/`**:
  - `web/src/pages/HomePage.tsx`: `viewMode` state + `localStorage`
    read/write; toggle control above the two sections; new table
    rendering branch for artists/albums, gated by `viewMode`.
  - `web/src/pages/HomePage.css` (or the shared `styles/catalog.css`,
    decided at implementation time): new `.data-table`/`.view-toggle`
    styles, reusing existing design tokens (`--border`, `--accent-wash`,
    etc.) rather than introducing new ones.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none.
- Update `docs/ARCHITECTURE.md` §3 (`web/` bullet: home page gains a
  grid/table toggle) once implementation lands.
