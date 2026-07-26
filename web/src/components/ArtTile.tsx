import { hasArtProblem, type ArtStatus } from '../api/library'

// ArtTile renders one artwork slot — a grid card's art/avatar, a detail
// page's hero, or a song row's thumbnail — as either the real image or a
// refined placeholder glyph, depending only on whether src is set. Shared
// across every page that shows artist/album art (TDR 003) so the
// img-vs-placeholder branching and the two icon glyphs live in one place.
// artStatus is optional (a song row's album thumbnail is the one context
// that always passes it, everywhere else does too since TDR 007) — when
// present and the lookup came up empty/errored with nothing to show, a
// small corner badge appears on the placeholder (TDR 007 AC-2).
interface ArtTileProps {
  src: string
  alt: string
  className: string
  kind: 'album' | 'artist'
  artStatus?: ArtStatus
}

export default function ArtTile({ src, alt, className, kind, artStatus }: ArtTileProps) {
  if (!src) {
    const problem = artStatus ? hasArtProblem(artStatus, src) : false
    return (
      <div className={`${className} placeholder`}>
        {kind === 'album' ? <AlbumGlyph /> : <ArtistGlyph />}
        {problem && (
          <span
            className={`art-badge ${artStatus === 'failed' ? 'bad' : 'warn'}`}
            title={artStatus === 'failed' ? 'Artwork lookup failed' : 'No artwork found'}
          />
        )}
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
