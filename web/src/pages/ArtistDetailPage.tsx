import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { errorMessage, getArtist, type ArtistDetail } from '../api/library'
import '../styles/catalog.css'

export default function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [artist, setArtist] = useState<ArtistDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setArtist(null)
    setLoadError(null)
    getArtist(Number(id))
      .then((result) => {
        if (!cancelled) setArtist(result)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [id])

  if (loadError) {
    return (
      <div className="page-shell">
        <p className="crumb">
          <Link to="/artists">Artists</Link>
        </p>
        <p className="library-load-error">{loadError}</p>
      </div>
    )
  }

  if (!artist) return null

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/artists">Artists</Link> / {artist.name || 'Unknown Artist'}
      </p>
      <div className="detail-head">
        <div className="detail-art round">
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="8" r="3.2" stroke="currentColor" strokeWidth="1.6" />
            <path d="M5 20c0-3.5 3-6 7-6s7 2.5 7 6" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
          </svg>
        </div>
        <div className="detail-meta">
          <div className="kind">Artist</div>
          <h1>{artist.name || 'Unknown Artist'}</h1>
          <div className="facts">
            {artist.albumCount} album{artist.albumCount === 1 ? '' : 's'} · {artist.trackCount} song
            {artist.trackCount === 1 ? '' : 's'}
          </div>
        </div>
      </div>

      <div className="section-head">
        <h2>Albums</h2>
      </div>
      <div className="card-grid">
        {artist.albums.map((album) => (
          <Link key={album.id} className="album-card" to={`/albums/${album.id}`}>
            <div className="art">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.6" />
                <circle cx="12" cy="12" r="2.5" stroke="currentColor" strokeWidth="1.6" />
              </svg>
            </div>
            <div className="title">{album.title || 'Unknown Album'}</div>
            <div className="artist">{album.year > 0 ? album.year : ''}</div>
          </Link>
        ))}
      </div>
    </div>
  )
}
