# User Story: Import History Hidden by Default

## 1. User Value Statement

As a **household member using the Import page regularly**, I want to **not
see a running history of every past import cluttering the page by default,
with a compact way to check it when I actually need to**, So that **the
Import page stays focused on starting a new import instead of scrolling
past a growing list of old ones**.

## 2. Strict Acceptance Criteria

- **AC-1**: On the Import page's list step (`/import`, `step === 'list'`),
  no import history is shown by default — not the list, not an empty-state
  message. Only a `Show import history` text link renders below the
  topbar, in the space the list occupies today.
- **AC-2**: Clicking `Show import history` reveals the history inline and
  the link's label changes to `Hide import history`; clicking it again
  collapses the list back to just the link.
- **AC-3**: The shown/hidden state persists across page loads and visits
  (stored in `localStorage`), defaulting to hidden the very first time.
- **AC-4**: When expanded, each import renders as a single compact line —
  status indicator, source description (truncated if long), track count,
  and a relative date — replacing today's multi-line card (path row,
  status pill, progress bar, meta row, separate error banner).
- **AC-5**: A failed import's error message is not shown inline as its own
  line; instead a small warning indicator on that row shows the full error
  text in a native tooltip on hover.
- **AC-6**: When expanded, at most the 10 most recent imports show
  initially; if there are more, a `Show N more` link reveals the rest
  (`N` = the remaining count).
- **AC-7**: If there are genuinely zero imports, expanding the toggle shows
  a short "no imports yet" line instead of the list — this line only
  appears when expanded, never in the collapsed state (AC-1).
- **AC-8**: No behavior changes to the "in-progress" copying view
  (`step === 'copying'`) or any other step — this only affects the list
  step's history section.
