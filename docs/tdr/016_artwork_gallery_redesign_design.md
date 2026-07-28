# TDR 016: Artwork Gallery Redesign

## 1. Context & Architectural Requirements

GitHub issue #40, "Artwork gallery redesign," carried no written spec — just
a title, filed by the household's primary user against the gallery TDR 014
shipped (grid of square tiles, one hero image up top duplicating the
grid's "primary" tile below it). Grilling it converged on two distinct,
independently-motivated complaints, not one:

1. **The gallery eats vertical space and pushes the actual album/artist
   content down the page.** The old grid grows without bound as more
   images accumulate (Cover Art Archive alone can return a dozen-plus
   typed images per release), and the hero-plus-grid layout shows the
   primary image twice.
2. **There's no way to inspect an image at real size.** A booklet scan's
   liner-note text is illegible at thumbnail size, and the old grid had no
   click-to-enlarge path at all.

A design mockup (Artifact, iterated live with the user across five
revisions) converged on the shape this TDR formalizes: a circular avatar
overlapping a banner image in the header (both drawn from the existing
gallery, not new uploads), and the full gallery below collapsed into a
single-image-at-a-time viewer with click-to-page navigation and a
full-size overlay for detail — see §2 for the specific alternatives the
mockup process ruled out along the way.

Current implementation (TDR 014), confirmed by reading the code:

- **Frontend**: `web/src/components/ArtworkGallery.tsx` renders every
  gallery image as a `gallery-grid` of fixed square tiles
  (`grid-template-columns: repeat(auto-fill, minmax(128px, 1fr))`); each
  tile has its own "Set primary"/"Remove" buttons and an always-present
  "+ Add" tile. `ArtistDetailPage.tsx`/`AlbumDetailPage.tsx` show this
  below a `.detail-head` hero (`ArtTile`, `catalog.css:375-392`) that
  renders only the primary image. `web/src/hooks/useEntityGallery.ts` is
  the shared data/action layer both detail pages configure via
  `EntityGalleryConfig`.
- **Backend**: `artist_photos`/`album_covers` (migration
  `0006_multiple_artworks.sql`) each carry a single `is_primary BOOLEAN`,
  enforced exactly-one-true-per-entity at the application layer by
  `Store.setGalleryPrimary` (`library/artwork_gallery.go:158-176`) — clear
  the entity's existing primary, then set the new one, in one transaction.
  `ArtistDetail`/`AlbumDetail`'s `PhotoThumbURL`/`PhotoURL` (and the album
  equivalents) are populated via a `LEFT JOIN LATERAL ... WHERE is_primary
  = true LIMIT 1` in `GetArtist`/`GetAlbum` (`catalog.go`). HTTP routes
  follow `POST /api/library/artists/{id}/photos/{photoId}/primary`
  (`httpserver.go:43,50`) and the DELETE equivalent.

There is no existing concept of a second, independently-chosen "banner"
image anywhere in this schema or API — this TDR adds one, deliberately
mirroring `is_primary`'s existing shape rather than inventing a new
pattern.

## 2. Alternatives Evaluated

### Alternative: how the header banner is derived

- **Color/gradient extracted from the primary image** (the "YouTube Music
  Android redesign" reference the user raised first) — Pros: needs no new
  image-selection concept at all, just a client-side palette computation
  over the existing primary image. Cons: rejected by the user after seeing
  it mocked up ("none are good") — a computed tint doesn't read as
  intentional the way a real second photograph does, and this app has no
  existing color-extraction dependency to build on.
- **Blurred/darkened version of the primary image itself** — Pros: also
  needs no new selection concept, simple CSS filter. Cons: also rejected
  in the same mockup round; still just one image doing two jobs, and a
  heavily blurred banner reads as generic rather than as real artwork.
- **A second, explicitly-chosen real image — same image as the avatar by
  default** — Pros: simplest data model (no new "which image" state,
  banner = primary, mirrors what real cover art actually looks like
  instead of an abstraction). Cons: rejected once mocked up — the user
  explicitly wants the avatar and banner able to differ ("avatar and
  banner should never be the same image"), e.g. a moody wide shot as
  banner with a cleaner square crop as the avatar.
- **A second, independently-settable image via a new per-image flag
  (chosen)** — mirrors `is_primary`'s existing exactly-one-per-entity
  shape as a second, independent flag (`is_banner`) on the same
  `artist_photos`/`album_covers` rows. Pros: reuses the schema pattern,
  the transactional set-flag helper, and the JOIN-for-detail-fetch
  approach already proven for `is_primary` — no new table, no new
  upload/selection UI beyond one more per-image action button. Cons: a
  second boolean invariant to maintain per entity (mitigated — see §3,
  it's the same generalized helper `is_primary` already uses).

### Alternative: browsing the rest of the gallery

- **Keep the wrapping grid, just make tiles smaller** — Pros: least
  change. Cons: still unbounded height as images accumulate; doesn't
  address either root complaint.
- **Horizontal thumbnail-strip carousel + click-to-open lightbox** (the
  first mockup revision) — Pros: bounded height regardless of image
  count. Cons: rejected by the user directly — "I don't want to see
  multiple artwork thumbnails" — a filmstrip is still a wall of images
  competing for attention, just laid out sideways instead of wrapped.
- **Single active image with click-to-page navigation (chosen)** — one
  image visible at a time, paged via clicking its left/right half (AC-5)
  or arrow keys; a small "expand" affordance opens a same-page full-screen
  overlay for detail (AC-7) instead of scaling content inside the small
  viewer. Pros: fixed, small footprint regardless of gallery size (directly
  answers complaint 1); a genuinely large view answers complaint 2 without
  the false precision of CSS-scaling a 144px box (an inline pinch/scroll
  zoom prototype was tried and explicitly rejected — "don't use zoom
  cursor — doesn't feel appropriate"). Cons: seeing the whole set at a
  glance takes more clicks than a grid — accepted; that trade is exactly
  what "don't want artwork taking up so much space" asked for.
- A `window.open()`-based "view in a new tab" variant of the full-size
  action was also tried in the mockup and dropped after it silently failed
  inside the sandboxed preview iframe; the in-page overlay chosen instead
  is also just a more broadly reliable pattern in production (no
  popup-blocker exposure, works the same on mobile web).

## 3. Structural Decision

### Schema (new migration `0007_artwork_banner.sql`)

```sql
ALTER TABLE artist_photos ADD COLUMN is_banner BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE album_covers  ADD COLUMN is_banner BOOLEAN NOT NULL DEFAULT FALSE;
```

No backfill: every existing row starts with `is_banner = false` for every
entity, which is exactly AC-3's "no banner chosen yet" state — the
COALESCE fallback below handles display until a reviewer picks one.
`is_banner` is enforced exactly-zero-or-one-true-per-entity at the
application layer, the same as `is_primary` — see below.

### Go/API shape

- `ArtistPhoto`/`AlbumCover` (`library/artwork_gallery.go`) gain
  `IsBanner bool `json:"isBanner"`` alongside the existing `IsPrimary`,
  populated by extending `listGalleryRows`'s column list.
- `Store.setGalleryPrimary` becomes `Store.setGalleryFlag(ctx, t
  galleryTable, entityID, imageID int64, column string, notFound error)
  error`, parameterizing the column name (`"is_primary"` or `"is_banner"`)
  the same way `t.name`/`t.fkColumn` are already interpolated —
  `SetArtistPrimaryPhoto`/`SetAlbumPrimaryCover` and the new
  `SetArtistBannerPhoto`/`SetAlbumBannerCover` become one-line callers of
  the shared helper with a different column, mirroring how
  `ListArtistPhotos`/`ListAlbumCovers` already share `listGalleryRows`.
- Unlike `is_primary`, deleting the image currently flagged `is_banner`
  does **not** auto-promote another image — `deleteGalleryRow`'s existing
  "promote oldest remaining to primary" logic (required because list
  tiles always need a primary image) has no banner equivalent, since a
  missing banner already has a defined, correct fallback (below). Simpler
  than adding a second auto-promotion path for a flag that doesn't need
  one.
- `ArtistDetail`/`AlbumDetail` (detail-fetch only, not list rows — same
  scoping `Photos []ArtistPhoto`/`Covers []AlbumCover` already use) gain
  `BannerURL string`. `GetArtist`/`GetAlbum` add a second `LEFT JOIN
  LATERAL (... WHERE is_banner = true LIMIT 1)` beside the existing
  primary-image join, and compute `BannerURL = COALESCE(banner_photo.
  full_path, primary_photo.full_path)` — AC-3's fallback lives entirely in
  this one SQL expression, no branching in Go or in the frontend. Uses
  each image's existing full-resolution variant (`ImageStore`'s
  `fullSize`, already 1000px) — no new image processing.
- New routes mirroring the existing primary ones exactly:
  `POST /api/library/artists/{id}/photos/{photoId}/banner`,
  `POST /api/library/albums/{id}/covers/{coverId}/banner`.

### Frontend shape

- `ArtistDetailPage.tsx`/`AlbumDetailPage.tsx`'s `.detail-head` hero
  (single `ArtTile`) is replaced by a banner-with-overlapping-avatar
  header: a full-bleed `BannerURL` image with a bottom scrim for text
  legibility, a circular avatar (`PhotoURL`/`CoverURL`, i.e. today's
  primary image, just re-shaped) overlapping its bottom edge, name/meta/
  actions beside it. `ArtTile`'s other call sites (grid tiles, song rows,
  artist chips) are untouched.
- `ArtworkGallery.tsx` is rewritten from a tile grid to a single-image
  viewer: one small fixed-size frame, click-left-half/click-right-half
  (and arrow keys) to page through `images`, a persistent low-opacity
  chevron per side as the discoverability cue (AC-5), an "expand" icon
  opening a same-page full-screen overlay (its own prev/next, Escape/
  click-outside/✕ to close) rather than scaling in place. Per-image
  actions become "Set as primary" / "Set as banner" / "Remove", each
  hidden on whichever image already holds that role.
- `useEntityGallery.ts`'s `EntityGalleryConfig` gains a `setBannerImage`
  entry alongside the existing `setPrimaryImage`, wired the same way.

## 4. Cross-Workspace Implications

- **`backend/`**: new migration `0007_artwork_banner.sql` (additive, no
  backfill); `library/artwork_gallery.go` (`IsBanner` field, generalized
  `setGalleryFlag`, new `SetArtistBannerPhoto`/`SetAlbumBannerCover`);
  `library/catalog.go` (`BannerURL` on `ArtistDetail`/`AlbumDetail`, second
  LATERAL join + COALESCE in `GetArtist`/`GetAlbum`); `httpserver.go` (two
  new `.../banner` routes) and `httpserver/catalog.go` (handlers, mirroring
  the existing primary ones). No new dependencies.
- **`web/`**: `ArtworkGallery.tsx` rewritten (mockup signed off — see
  conversation Artifact history for the five-revision process); detail
  page header markup in `ArtistDetailPage.tsx`/`AlbumDetailPage.tsx` and
  shared `styles/catalog.css`; `useEntityGallery.ts` gains
  `setBannerImage`. No new dependencies — the full-size overlay and
  click-to-page zones are plain CSS/DOM, no carousel/lightbox library.
- **`mobile/`**: out of scope, unchanged.
- **Schema**: `artist_photos.is_banner`/`album_covers.is_banner` added,
  additive and backward compatible (defaults false, existing rows
  unaffected, `BannerURL`'s COALESCE fallback means no entity ever shows a
  broken header even with zero rows flagged).
- Update `docs/ARCHITECTURE.md`'s artwork section once this lands.
