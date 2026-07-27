# User Story: Exclude Tracks from Import

## 1. User Value Statement

As a **household member reviewing an import plan**, I want to **uncheck
individual tracks (or a whole album) I don't want copied**, So that **I
don't have to import everything found in a source just to get the few
tracks I actually want, or restart the whole import to drop one bad file**.

## 2. Strict Acceptance Criteria

- **AC-1**: Every track row in the review step (`step === 'review'`) has a
  checkbox, checked by default. Unchecking it visually marks the row as
  excluded (dimmed/struck-through) — the row stays in place and can be
  re-checked at any time.
- **AC-2**: Each album group header has its own checkbox that selects or
  deselects every track in that album at once; it reflects mixed state
  (some but not all tracks checked) rather than forcing a snap to fully
  checked or unchecked.
- **AC-3**: An album with zero tracks currently checked stays visible in
  the review list — its artist/album/year fields remain editable — showing
  a "0 tracks selected" indicator in place of the normal ready/attention
  status for that album.
- **AC-4**: An excluded track's missing-field warnings and destination
  conflicts are fully suppressed: it doesn't render a conflict-resolution
  row (`Change track #` / `Overwrite existing`), and its errors don't count
  toward the "N tracks need attention" banner.
- **AC-5**: `Confirm & import N tracks"` counts only currently-included
  tracks across every album. If that count is 0 (everything excluded),
  the button is disabled.
- **AC-6**: Excluded tracks are never sent to `validatePlan` or
  `confirmImport` — filtering happens purely client-side before either
  call, so unchecking a track requires no extra network round-trip and no
  backend change.
