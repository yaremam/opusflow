# TDR 010: Import History Hidden by Default

## 1. Context & Architectural Requirements

GitHub issue #15 asked for the Import page's history list to not show by
default, and to be "much more compact" when shown. Today, `ImportPage.tsx`'s
`'list'` step always renders every import returned by `listImports()`
(newest first, per `TestListImportsNewestFirst`) as a multi-line card: a
path row with a status pill, a progress bar for `'copying'` imports, a meta
row (track count + date), and — for `'failed'` imports — a separate red
error-banner line below the card.

Grilling this surfaced a fact worth recording: in the current single-tab,
synchronous workflow, the list step is never showing a `'copying'` import
in practice — confirming an import immediately transitions the whole page
to the `'copying'` step (`setStep('copying')`) and then to `'done'`; a user
only lands back on `'list'` after that import has already finished. So this
feature doesn't need to special-case "in-progress" vs "history" — every
entry the list step ever shows is, from the user's point of view, already
history.

This is a purely frontend, purely presentational change: no new data, no
new endpoint, no schema change — `listImports()`'s response already has
everything the compact format needs.

## 2. Alternatives Evaluated

### Alternative: where the hidden/shown state lives — localStorage vs component state vs a backend setting

- **Component state only** — Pros: simplest possible change. Cons: resets
  to (per AC-3, hidden) every page load, so a user who wants to glance at
  history regularly has to re-expand it every visit — annoying, and this
  app already has precedent (TDR 008's `viewMode`) for using `localStorage`
  instead for exactly this kind of standing UI preference.
- **A backend-stored per-library/user setting** — Pros: would survive
  across browsers/devices. Cons: there's no user-accounts/preferences
  system in this app at all; introducing one for a single boolean is wildly
  disproportionate, and the household/single-device usage pattern this app
  targets doesn't need cross-device sync for a UI preference.
- **`localStorage` (chosen)** — Pros: persists across reloads/visits with
  zero backend involvement, consistent with the precedent TDR 008 already
  set for "remembered but not synced" UI state. Cons: local to one
  browser/device — accepted, same trade-off TDR 008 already made.

### Alternative: surfacing a failed import's error in the compact row — hover tooltip vs click-to-expand vs always-visible line

- **Always-visible line (today's behavior)** — Pros: error is impossible to
  miss. Cons: this is exactly the multi-line bulk AC-4 is trying to
  eliminate — one failed import would still take two lines, defeating the
  "much more compact" goal.
- **Click-to-expand** — Pros: keeps the row collapsed until the user asks
  for detail, no hover dependency (works identically on touch). Cons: adds
  interaction state (which row is expanded) and a second visual mode to a
  component that's supposed to be simpler than what it replaces; a history
  list is a "glance at it, maybe hover" surface, not a place people expect
  to drill in.
- **Hover tooltip (chosen)** — Pros: zero layout cost — the row stays a
  single line whether or not it failed; a native `title` attribute needs no
  new component or state. Cons: not reachable by touch/keyboard-only users
  without a fallback; acceptable here since the row itself remains a normal
  focusable element and the failure is still visible at a glance via the
  warning indicator's color/icon, same severity signal as today's pill.

### Alternative: bounding an ever-growing list — cap with "show more" vs show everything vs paginate via API

- **Show everything** — Pros: simplest, no extra control. Cons: directly
  works against the "much more compact" goal after months of regular use —
  exactly the unbounded growth the issue is reacting to in the first place.
- **Server-side pagination (new `?limit`/`?offset` on `GET /api/imports`)**
  — Pros: scales indefinitely, never fetches more than needed. Cons: a new
  API contract change for a page that, per the issue, people mostly want to
  *not* look at; premature for a household-scale import count.
- **Client-side cap with "Show N more" (chosen)** — Pros: no backend
  change — `listImports()` already returns everything, this just slices
  what renders; matches AC-6 exactly. Cons: still fetches the full list
  from the API regardless of how much renders — acceptable at household
  scale (dozens to low hundreds of imports, not millions).

## 3. Structural Decision

`ImportPage.tsx` gains a `historyExpanded` boolean, initialized from a
`localStorage` key (e.g. `opusflow.importHistoryExpanded`) and written back
on toggle — the same `useState`+`useEffect` pattern TDR 008 established for
`viewMode`, applied to a second, independent preference.

The `'list'` step's rendering splits into:
- The `Show import history` / `Hide import history` link, always present
  once `imports` has loaded.
- When `historyExpanded`, either the compact list or a short "no imports
  yet" line (AC-7) if `imports.length === 0`.

The compact list replaces each `library-card` with a single-line row:
status indicator (color/icon reused from today's status pill), truncated
`sourceDescription`, track count, and a relative date (reusing the existing
`formatDate` helper, or a new relative-time variant if the grilled mockup
calls for it). Rows past the 10th are held back behind a `visibleCount`
state, with a `Show N more` link bumping it to `imports.length`. A failed
row's warning indicator carries the error text in its `title` attribute
instead of today's separate `.error-note` line.

## 4. Cross-Workspace Implications

- **`backend/`**: untouched — no new fields, no new endpoint, no schema
  change. `listImports()` (`web/src/api/library.ts`) already returns every
  field the compact row needs (`sourceDescription`, `status`, `trackCount`,
  `createdAt`, `error`).
- **`web/`**:
  - `web/src/pages/ImportPage.tsx`: `historyExpanded` + `visibleCount`
    state, `localStorage` read/write for the former; the `'list'` step's
    JSX reworked per §3.
  - `web/src/pages/ImportPage.css`: new compact-row styles (single-line
    layout, status dot, truncated text), reusing existing tokens
    (`--good`/`--warn`/`--bad`, `--border`, `--mist-400`) rather than
    introducing new ones; today's `.library-card`/`.progress-track`/
    `.error-note` styles can be removed once nothing references them.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none.
- Update `docs/ARCHITECTURE.md` if the Import page's description there
  calls out the always-visible history list by name (a quick grep at
  implementation time will confirm either way).
