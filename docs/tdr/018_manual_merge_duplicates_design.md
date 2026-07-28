# TDR 018: Manual Merge Tool for Duplicate Artists/Albums

## 1. Context & Architectural Requirements

GitHub issue #31, "Manual tool to merge duplicate artists / albums"
(enhancement, no further written spec) — the manual counterpart to issue
#30's automatic MusicBrainz-ID dedup: for a duplicate MusicBrainz can't
resolve at all, or one that existed before TDR 017 shipped and will never
be revisited automatically (see TDR 017 §1's scope note), a household
member needs a way to fix it themselves.

Since this is new interactive UI, it went through this repo's mockup
process (`CLAUDE.md`): a single mockup (Artifact, five-revision iteration
was TDR 016's process, not needed here — the interaction shape was
well-precedented enough to converge in one pass) built by reusing two
existing patterns rather than inventing new ones:

- `MetadataLookupModal`'s search → step → result-card visual vocabulary
  (`mdl-*` CSS classes) for the "search and pick" step.
- `RemoveModal`'s stance on a destructive, unrecoverable action: name
  exactly what happens, then require an explicit confirming click — no
  silent defaults.

Confirmed by reading the code: no rename/merge endpoint of any kind
exists on `artists`/`albums` today (only delete, art retry/upload, and
gallery primary/delete); no multi-select or bulk-action UI exists
anywhere in this app's index pages (`ArtistsPage.tsx`, `AlbumsPage.tsx`,
`SongsPage.tsx` are plain filtered card grids). A merge is the first
"combine two catalog rows" mutation this app has.

## 2. Alternatives Evaluated

### Alternative: where the merge is triggered from

- **Index-page checkbox multi-select + toolbar action** — Pros: lets a
  user compare several candidates side by side before picking a pair.
  Cons: no selection-state precedent anywhere in this app to build on;
  meaningfully more UI surface (checkboxes, a selection toolbar) for a
  household-scale library where duplicates are rare, one-off events, not
  a bulk-cleanup workflow.
- **A dedicated "Manage duplicates" utility page** — Pros: could
  proactively group same/similar names together. Cons: needs its own
  detection heuristic (fuzzy name matching) this app doesn't have and
  issue #30 already covers the "detect via MusicBrainz" case; a whole new
  page for what's meant to be an occasional manual fix is disproportionate
  scope.
- **"Merge into…" action on the entity's own detail page (chosen)** —
  Pros: matches the existing "Remove…" action's placement and framing
  exactly (a self-referential action on the row you're currently looking
  at, then you're navigated away once it resolves) — the user already
  knows this pattern from removal. No new page, no new selection-state
  machinery. Cons: only supports merging one pair at a time, starting
  from one of the two rows — acceptable; that's the actual use case
  (noticing one duplicate while looking at it).

### Alternative: how the target row is chosen

- **Free-text rename/alias field** — rejected outright; doesn't merge
  anything, just relabels one row, leaving the underlying duplication
  (two rows, two sets of albums/tracks) untouched.
- **Search-and-pick from the user's own local catalog (chosen)** —
  mirrors `MetadataLookupModal`'s already-proven search/result-card
  pattern, just searching local `ListArtists`/`ListAlbums` (which already
  support a `q` filter) instead of MusicBrainz. No new backend search
  endpoint needed.

## 3. Structural Decision

### Backend

`library.Service` gains thin wrappers over TDR 017's already-built
`Store.MergeArtists`/`MergeAlbums` (`backend/internal/library/merge.go` —
built for TDR 017, reused verbatim here per AC-5):

```go
func (s *Service) MergeArtists(ctx context.Context, id, intoID int64) error
func (s *Service) MergeAlbums(ctx context.Context, id, intoID int64) error
```

New routes, mirroring the existing delete endpoints' shape:

```
POST /api/library/artists/{id}/merge   body: {"intoId": <int64>}
POST /api/library/albums/{id}/merge    body: {"intoId": <int64>}
```

`id` is always the row that stops existing; `intoId` (the request body,
not the path — the path identifies "the page you're merging away from")
is the one that survives. Both return the survivor's fresh detail
(`ArtistDetail`/`AlbumDetail`) on success, matching how
`handleSetGalleryPrimary` already returns fresh detail rather than a bare
204 — the frontend navigates straight there. `library.ErrCannotMergeIntoSelf`
(added to `merge.go` alongside TDR 017's other sentinels) and
`ErrAlbumsBelongToDifferentArtists` map to `400 Bad Request` via
`libraryErrorStatus`; `ErrArtistNotFound`/`ErrAlbumNotFound` (already
mapped) cover an `intoId` that doesn't exist.

### Frontend

New `MergeModal` component (`web/src/components/MergeModal.tsx` +
`.css`, the `.css` intentionally duplicating `MetadataLookupModal.css`'s
`mdl-*` rules rather than importing them — this codebase already keeps
one stylesheet per component, e.g. `ArtworkGallery.css`/`RemoveModal.css`,
so this follows suit instead of introducing cross-component CSS coupling)
is generic over "artist" or "album": the caller supplies a `search`
function (wrapping `listArtists`/`listAlbums`, filtering out the source
row and — for albums — every row not under the same artist, per AC-3)
and a `merge` function (wrapping `mergeArtist`/`mergeAlbum`), plus the
merge-specific effects bullets to show at confirm time (the component
itself only owns the generic "remove the source, can't be undone" line).

`ArtistDetailPage.tsx`/`AlbumDetailPage.tsx` each add a "Merge into…"
button beside "Remove…" (new shared `.detail-secondary-actions` flex
row in `catalog.css`) opening `MergeModal`; on success, `onMerged`
closes the modal *and* navigates to `/artists/{intoId}` (or
`/albums/{intoId}`) — both matter, since React Router doesn't remount
the page component on a path-param change alone, so skipping the
modal-close would leave it open, now bound to the just-kept row's own
data (caught during manual verification against the real running app,
fixed before merging).

## 4. Cross-Workspace Implications

- **`backend/`**: `library/service.go` (two wrapper methods, extends
  `CatalogReader`); `library/merge.go` (two new sentinel errors, reusing
  TDR 017's `MergeArtists`/`MergeAlbums`/`mergeAlbumRows`); `httpserver/catalog.go`
  (two new handlers) and `httpserver.go` (two new routes);
  `httpserver/imports.go`'s `libraryErrorStatus` (two new mappings). No
  schema change, no new dependencies.
- **`web/`**: new `components/MergeModal.tsx`/`.css`; `api/library.ts`
  (`mergeArtist`/`mergeAlbum`); `pages/ArtistDetailPage.tsx`/
  `AlbumDetailPage.tsx` (trigger button + modal wiring);
  `styles/catalog.css` (`.detail-secondary-actions`). No new
  dependencies — the modal is plain React state, no new library.
- **`mobile/`**: out of scope, unchanged.
- Depends on TDR 017 (issue #30) for `MergeArtists`/`MergeAlbums`
  themselves — this entry only adds the manual trigger around them.
