# User Story: Artist & Album Artwork and Info

## 1. User Value Statement

As a **household member browsing the library**, I want to **see real cover
art, artist photos, and a bit of context about each artist and album**, So
that **Artists, Albums, and Songs feel like a real music app instead of a
grid of placeholder icons and bare titles**.

## 2. Strict Acceptance Criteria

### Artwork sourcing

- **AC-1**: When a track's audio file carries embedded cover art (ID3 APIC /
  FLAC picture block / MP4 cover atom), that image is extracted and used as
  its album's art. An album with multiple tracks uses the first embedded
  image found across them.
- **AC-2**: For any album still without art after AC-1, and for every
  artist, a background job looks up art via MusicBrainz: album covers
  through the Cover Art Archive (by matched release), artist photos through
  the MusicBrainz → Wikidata (`P18`) → Wikimedia Commons chain. The
  top-ranked MusicBrainz search result is used; no confidence threshold.
- **AC-3**: Artists/albums with an empty ("Unknown Artist" / "Unknown
  Album") name are never looked up — AC-1/AC-2 are skipped for them and
  they always render the placeholder tile (AC-9).
- **AC-4**: The background job is not scoped to a single scan — each run, it
  processes every artist/album whose art status is `pending` or `failed`,
  regardless of when they were added. It runs once when the backend starts,
  and again after every directory scan completes.
- **AC-5**: Each artist and album tracks an independent art status:
  `pending` (never looked at), `found`, `not_found` (MusicBrainz/Cover Art
  Archive/Commons responded with no match — not retried), or `failed`
  (network/rate-limit error — retried on the next job run).

### Info sourcing (facts + bio)

- **AC-6**: The same background job resolves structured facts from
  MusicBrainz: for an artist, formed/born year, country, and genre tags; for
  an album, label, country, and genre tags. Facts are shown as a row of
  chips on the respective detail page when present.
- **AC-7**: Where the matched MusicBrainz entity links to a Wikidata item
  with a Wikipedia sitelink, a short bio (artist) or description (album) is
  fetched from that Wikipedia article's summary and shown as a paragraph on
  the detail page, with a visible "via Wikipedia" attribution linking to the
  source article.
- **AC-8**: Facts and bio/description each track their own independent
  status (same `pending`/`found`/`not_found`/`failed` states as AC-5) per
  artist/album — an item can show facts with no bio, a bio with no facts, or
  any other combination, rather than an all-or-nothing lookup.

### Rendering

- **AC-9**: Wherever an album or artist without art is rendered (Albums
  grid, Artists grid, Album detail hero, Artist detail hero, Artist detail's
  album grid, Songs list row thumbnail), a refined placeholder tile is shown
  instead of a broken image — never an empty box or a raw missing-image
  icon.
- **AC-10**: The Albums index, Artists index, and Artist detail's album grid
  render each item's album/artist art in its card in place of today's
  circle-icon tile.
- **AC-11**: The Album detail and Artist detail pages render their subject's
  art as a large hero image/photo in place of today's circle-icon tile.
- **AC-12**: The Songs index renders a small album-art thumbnail on each row.
- **AC-13**: The Album detail page shows the album's fact chips (AC-6) and
  description (AC-7) when present; the Artist detail page shows the
  artist's fact chips and bio the same way. Neither section renders at all
  when nothing was found for that kind (no empty chip row, no empty
  paragraph).

### Storage & image handling

- **AC-14**: Extracted/fetched images are stored as files (not database
  blobs); the artist/album row stores a reference to the stored file, not
  the bytes.
- **AC-15**: Each stored image is saved in two sizes — a grid-thumbnail
  variant and a larger detail-page variant — generated at ingest time.
