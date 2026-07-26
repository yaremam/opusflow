import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import { deleteAlbum, errorMessage, formatDuration, getAlbum, type AlbumDetail } from '../api/library'
import ArtTile from '../components/ArtTile'
import InfoBlock from '../components/InfoBlock'
import RemoveModal from '../components/RemoveModal'
import '../styles/catalog.css'

export default function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [album, setAlbum] = useState<AlbumDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [removing, setRemoving] = useState(false)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)

  async function handleRemove(deleteFiles: boolean) {
    if (!album) return
    setRemoveSubmitting(true)
    setRemoveError(null)
    try {
      await deleteAlbum(album.id, deleteFiles)
      navigate('/albums')
    } catch (err) {
      setRemoveError(errorMessage(err))
      setRemoveSubmitting(false)
    }
  }

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
        <ArtTile src={album.coverUrl || album.coverThumbUrl} alt="" className="detail-art" kind="album" />
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
          <button type="button" className="btn-ghost detail-remove" onClick={() => setRemoving(true)}>
            Remove album…
          </button>
        </div>
      </div>

      {removing && (
        <RemoveModal
          name={album.title || 'Unknown Album'}
          submitting={removeSubmitting}
          submitError={removeError}
          onDeleteFiles={() => handleRemove(true)}
          onKeepFiles={() => handleRemove(false)}
          onCancel={() => {
            setRemoving(false)
            setRemoveError(null)
          }}
        />
      )}

      <InfoBlock
        facts={[
          ...(album.label ? [{ label: 'Label', value: album.label }] : []),
          ...(album.country ? [{ label: 'Country', value: album.country }] : []),
          ...album.genres.map((g) => ({ label: 'Genre', value: g })),
        ]}
        text={album.description}
        sourceUrl={album.descriptionSourceUrl}
      />

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
