// ArtTile renders one artwork slot — a grid card's art/avatar, a detail
// page's hero, or a song row's thumbnail — as either the real image or a
// refined placeholder glyph, depending only on whether src is set. Shared
// across every page that shows artist/album art (TDR 003) so the
// img-vs-placeholder branching and the two icon glyphs live in one place.
interface ArtTileProps {
  src: string
  alt: string
  className: string
  kind: 'album' | 'artist'
}

export default function ArtTile({ src, alt, className, kind }: ArtTileProps) {
  if (!src) {
    return (
      <div className={`${className} placeholder`}>
        {kind === 'album' ? <AlbumGlyph /> : <ArtistGlyph />}
      </div>
    )
  }
  return (
    <div className={className}>
      <img src={src} alt={alt} loading="lazy" />
    </div>
  )
}

function AlbumGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="12" cy="12" r="3" fill="currentColor" />
      <circle cx="12" cy="12" r="1" fill="var(--ink-800)" />
    </svg>
  )
}

function ArtistGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <circle cx="12" cy="8.2" r="3.4" stroke="currentColor" strokeWidth="1.5" />
      <path d="M4.6 20c0-4 3.3-6.8 7.4-6.8s7.4 2.8 7.4 6.8" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  )
}
