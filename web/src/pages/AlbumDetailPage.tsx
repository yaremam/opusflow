import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { errorMessage, formatDuration, getAlbum, type AlbumDetail } from '../api/library'
import '../styles/catalog.css'

export default function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const [album, setAlbum] = useState<AlbumDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setAlbum(null)
    setLoadError(null)
    getAlbum(Number(id))
      .then((result) => {
        if (!cancelled) setAlbum(result)
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
          <Link to="/albums">Albums</Link>
        </p>
        <p className="library-load-error">{loadError}</p>
      </div>
    )
  }

  if (!album) return null

  const totalSeconds = album.tracks.reduce((sum, t) => sum + t.durationSeconds, 0)

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/albums">Albums</Link> / {album.title || 'Unknown Album'}
      </p>
      <div className="detail-head">
        <div className="detail-art">
          <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="1.6" />
            <circle cx="12" cy="12" r="2.5" stroke="currentColor" strokeWidth="1.6" />
          </svg>
        </div>
        <div className="detail-meta">
          <div className="kind">Album</div>
          <h1>{album.title || 'Unknown Album'}</h1>
          <div className="by">
            by <Link to={`/artists/${album.artistId}`}>{album.artistName || 'Unknown Artist'}</Link>
          </div>
          <div className="facts">
            {album.year > 0 ? `${album.year} · ` : ''}
            {album.trackCount} song{album.trackCount === 1 ? '' : 's'} · {formatDuration(totalSeconds)}
          </div>
        </div>
      </div>

      <table className="track-table">
        <thead>
          <tr>
            <th className="num">#</th>
            <th>Title</th>
            <th className="num">Duration</th>
          </tr>
        </thead>
        <tbody>
          {album.tracks.map((track) => (
            <tr key={track.id}>
              <td className="trk">{track.trackNumber || ''}</td>
              <td className="t">{track.title}</td>
              <td className="dur">{formatDuration(track.durationSeconds)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
