import './PlaylistCoverTile.css'

interface PlaylistCoverTileProps {
  coverUrls: string[]
  className: string
}

// PlaylistCoverTile renders a playlist's cover as a 2x2 collage of its
// first up to 4 tracks' album art (AC-7) — ArtTile's sibling, not a
// variant of it: a playlist has no art of its own to fetch/retry, just a
// derived collage from whatever's already in the catalog, so an empty
// playlist (or one whose tracks have no covers yet) falls back to a
// placeholder glyph exactly like ArtTile's does.
export default function PlaylistCoverTile({ coverUrls, className }: PlaylistCoverTileProps) {
  if (coverUrls.length === 0) {
    return (
      <div className={`${className} playlist-cover placeholder`}>
        <NoteGlyph />
      </div>
    )
  }
  return (
    <div className={`${className} playlist-cover`}>
      {Array.from({ length: 4 }).map((_, i) => (
        <div className="cell" key={i}>
          {coverUrls[i] && <img src={coverUrls[i]} alt="" loading="lazy" />}
        </div>
      ))}
    </div>
  )
}

function NoteGlyph() {
  return (
    <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M9 18V5l10-2v12" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="6" cy="18" r="3" stroke="currentColor" strokeWidth="1.5" />
      <circle cx="16" cy="15" r="3" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  )
}
