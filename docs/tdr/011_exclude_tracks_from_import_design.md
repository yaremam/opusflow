# TDR 011: Exclude Tracks from Import

## 1. Context & Architectural Requirements

GitHub issue #16 asked for the ability to skip individual tracks when
reviewing an import plan (`ImportPage.tsx`, `step === 'review'`). Today
every track `BuildPlan`/`buildPlan` finds gets sent through `validatePlan`
(on every field edit) and, eventually, `confirmImport` — there's no concept
of "don't copy this one."

Checking the backend confirms this needs no server-side change at all:
`organize.Copy` (`backend/internal/library/organize/copy.go:72`) simply
iterates whatever `plan.Albums[].Tracks` it's handed — `total` (used for
progress reporting) is computed from the plan itself, and nothing
cross-checks the plan against the original source directory listing. A
track that never appears in the `Plan` payload sent to `confirmImport` is,
as far as the backend is concerned, a track that was never found — exactly
the behavior exclusion needs.

## 2. Alternatives Evaluated

### Alternative: where "excluded" state lives — a `PlanTrack` field vs. separate frontend-only state

- **Add an `excluded` field to `PlanTrack`** — Pros: travels naturally
  alongside the rest of a track's editable state (`trackNumber`, `title`,
  `overwrite`), one shape to reason about. Cons: `PlanTrack` is the exact
  JSON shape `buildPlan`/`validatePlan`/`confirmImport` send and receive —
  Go's `organize.Track` struct would need a matching field even though the
  backend would only ever see it stripped to `false`/absent (AC-6 requires
  excluded tracks never reach the backend at all), so the field would exist
  on both sides purely for the frontend's benefit. That's a wire-format
  change for a concept the backend is explicitly not supposed to know
  about.
- **Separate frontend-only state (chosen)** — a
  `Set<string>` of `${albumIndex}:${trackIndex}` keys (the same composite
  key `trackNumberRefs` and `errorFor` already use), living in `ImportPage`
  alongside `plan`/`errors`. Pros: zero backend/API changes — `PlanTrack`,
  `organize.Track`, and every existing test that constructs one is
  untouched; matches AC-6 exactly, since "excluded" never needs to survive
  a round-trip through JSON. Cons: one more piece of parallel state to keep
  in sync with `plan.Albums[].Tracks` indices — acceptable, since indices
  are already how this component tracks per-row error/ref state today and
  the array order doesn't change after the plan is built.

### Alternative: per-album bulk control — tri-state checkbox vs. a text action (matching TDR-013's "Overwrite all" button)

- **Text action, e.g. "Select all" / "Select none"** — Pros: consistent
  with the `Overwrite all N existing` button pattern already shipped for
  destination conflicts. Cons: needs two different labels/actions
  depending on current state (or a single label that's ambiguous about
  which way it'll flip), and doesn't visually communicate "some but not all
  are selected" the way a checkbox's indeterminate state does natively.
- **Tri-state checkbox in the album header (chosen)** — a real
  `<input type="checkbox">` with its `indeterminate` DOM property set
  (via a ref, since React has no `indeterminate` prop) when some but not
  all of an album's tracks are checked; clicking it either checks or
  unchecks every track in the album. Pros: one control, native
  checked/unchecked/indeterminate states map exactly to the three
  situations (all in, all out, mixed) with no extra copy needed. Cons:
  `indeterminate` needs a small `useEffect`/ref (it's DOM-property-only,
  not a JSX attribute) — a well-known, contained pattern, not a real
  downside.

## 3. Structural Decision

`ImportPage` gains an `excludedTracks` state: `Set<string>` of
`${albumIndex}:${trackIndex}` keys, with `toggleTrackExcluded(albumIndex,
trackIndex)` and `toggleAlbumExcluded(albumIndex, allExcluded)` mutators.
Nothing here touches `plan` or triggers `revalidate` — exclusion is a
purely presentational/filtering concern, so toggling a checkbox is
synchronous local state, no network round-trip (AC-6).

Three read-time helpers, all pure functions of `plan` + `excludedTracks`,
used both for rendering and for building request payloads:
- `isExcluded(albumIndex, trackIndex)` — membership check.
- `albumSelection(albumIndex)` — `'all' | 'none' | 'mixed'`, drives the
  header checkbox's `checked`/`indeterminate` props and the "0 tracks
  selected" indicator (AC-3).
- `includedPlan(plan, excludedTracks)` — returns a `Plan` with excluded
  tracks filtered out of every album's `Tracks` array (never removing the
  album itself, per AC-3). This is what gets sent to `validatePlan` (on
  blur/overwrite/track-number-change handlers) and `confirmImport` instead
  of raw `plan` — since excluded tracks never reach either call, their
  missing-field/conflict errors can never appear in the `errors` array the
  backend returns (AC-4), and `totalTracks`/the Confirm button's disabled
  state (AC-5) are computed from `includedPlan(...)`'s track count rather
  than `plan`'s.

Rendering: each track `<tr>` gains a leading checkbox `<td>` bound to
`toggleTrackExcluded`; excluded rows get an `excluded` class (opacity +
strikethrough via CSS, no JSX branching needed). The conflict-resolution
`<tr>` (today gated on `isConflict && !tr.overwrite`) gains `&&
!isExcluded(...)` (AC-4). The album header gets the tri-state checkbox
next to the artist/album fields.

## 4. Cross-Workspace Implications

- **`backend/`**: untouched — no new fields on `organize.Track`/`Plan`, no
  endpoint change. Confirmed by reading `organize.Copy` (§1): it already
  only ever sees whatever tracks are in the `Plan` it's handed.
- **`web/`**:
  - `web/src/pages/ImportPage.tsx`: `excludedTracks` state + the three
    helpers above; per-track and per-album checkboxes; `includedPlan(...)`
    substituted everywhere `plan` was being sent to `validatePlan`/
    `confirmImport`; `totalTracks`/`hasErrors`/the Confirm button's
    `disabled` computed against `includedPlan(...)` instead of `plan`.
  - `web/src/pages/ImportPage.css`: `.excluded` row styling (dim + line-
    through), tri-state checkbox sizing consistent with existing
    `field-inline`/table styles.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: none — `PlanTrack`/`organize.Track` are unchanged.
