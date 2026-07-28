# User Story: Artwork Gallery Redesign

## 1. User Value Statement

As a **household member browsing artist and album pages**, I want to **see
a compact, framed header built from artwork I've chosen, and step through
the rest of an entity's images one at a time without a wall of thumbnails
pushing the actual song list down the page**, So that **the artwork
enhances the page instead of dominating it, while I can still open any
single image full-size when I actually want to inspect detail — like
reading a booklet scan**.

## 2. Strict Acceptance Criteria

- **AC-1**: An artist/album detail page header shows a small circular
  avatar overlapping a wide banner image. The avatar is the entity's
  existing primary image (TDR 014); the banner is a second,
  independently-chosen image — never required to be the same image as the
  avatar, and never derived by blurring/recoloring the avatar.
- **AC-2**: Any image in the entity's gallery can be set as the avatar
  ("Set as primary", reusing TDR 014's existing primary concept) or as the
  banner ("Set as banner") via two independent actions. Setting one has no
  effect on the other; the same image is allowed to hold both roles at
  once if the reviewer chooses that.
- **AC-3**: An entity with no image yet explicitly set as banner (every
  existing artist/album immediately after this ships, since the concept
  is new) shows its primary image as the banner as a fallback, rather than
  a blank/flat header.
- **AC-4**: Below the header, the entity's full artwork gallery — every
  image, not just avatar/banner — is browsed one image at a time in a
  small, fixed-size viewer (not a wrapping grid, not a horizontal
  thumbnail strip).
- **AC-5**: The viewer navigates via clicking the left or right half of
  the currently shown image (previous/next respectively) and via the
  left/right arrow keys. Each half shows a small, low-opacity chevron at
  rest that brightens on hover/focus, so the interaction is discoverable
  without requiring a hover first.
- **AC-6**: Each image in the viewer keeps its existing per-image actions
  — set as primary, set as banner, remove (reusing the existing
  keep-vs-delete-file confirmation) — plus the existing "Add" action to
  upload a new image into the gallery.
- **AC-7**: An "expand" action opens the currently-shown image in a large
  in-page overlay for inspecting fine detail, with its own left/right
  paging. The overlay closes via a close button, clicking its background,
  or Escape.
- **AC-8**: No pinch/scroll/drag zoom interaction and no "zoom" cursor
  appear anywhere in the redesigned artwork section — magnifying an image
  is achieved solely by opening AC-7's full-size overlay, never by scaling
  content inside the small inline viewer.
- **AC-9**: This redesign applies uniformly to both artist pages (photo
  gallery) and album pages (cover gallery), consistent with the existing
  shared `ArtworkGallery`/`useEntityGallery` architecture (TDR 014) — no
  artist-only or album-only special casing.
