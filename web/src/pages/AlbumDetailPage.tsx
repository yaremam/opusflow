import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deleteAlbum,
  deleteAlbumCover,
  errorMessage,
  formatDuration,
  getAlbum,
  pollForArtResolution,
  retryAlbumArt,
  setAlbumPrimaryCover,
  uploadAlbumArt,
  type AlbumDetail,
  type AlbumTrack,
} from '../api/library'
import ArtActions from '../components/ArtActions'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import InfoBlock from '../components/InfoBlock'
import PlayButton from '../components/PlayButton'
import RemoveModal from '../components/RemoveModal'
import type { PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import '../styles/catalog.css'

export default function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [album, setAlbum] = useState<AlbumDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [removing, setRemoving] = useState(false)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)
  const player = usePlayer()

  function toPlayableTrack(track: AlbumTrack): PlayableTrack {
    return {
      id: track.id,
      title: track.title,
      artistName: album?.artistName ?? '',
      albumTitle: album?.title ?? '',
      albumCoverThumbUrl: album?.coverThumbUrl ?? '',
      durationSeconds: track.durationSeconds,
      format: track.format,
    }
  }

  function handlePlay(index: number) {
    if (!album) return
    player.playFrom(album.tracks.map(toPlayableTrack), index)
  }

  async function handleRetryArt() {
    if (!album) return
    const queued = await retryAlbumArt(album.id)
    setAlbum((prev) => prev && { ...prev, ...queued })
    const resolved = await pollForArtResolution(() => getAlbum(album.id))
    setAlbum((prev) => prev && { ...prev, ...resolved })
  }

  async function handleUploadCover(file: File) {
    if (!album) return
    const updated = await uploadAlbumArt(album.id, file)
    setAlbum(updated)
  }

  async function handleSetPrimaryCover(coverId: number) {
    if (!album) return
    const updated = await setAlbumPrimaryCover(album.id, coverId)
    setAlbum(updated)
  }

  async function handleDeleteCover(coverId: number, deleteFile: boolean) {
    if (!album) return
    const updated = await deleteAlbumCover(album.id, coverId, deleteFile)
    setAlbum(updated)
  }

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
        <ArtTile
          src={album.coverUrl || album.coverThumbUrl}
          alt=""
          className="detail-art"
          kind="album"
          artStatus={album.artStatus}
        />
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
          <ArtActions thumbUrl={album.coverThumbUrl} artStatus={album.artStatus} onRetry={handleRetryArt} />
          <button type="button" className="btn-ghost detail-remove" onClick={() => setRemoving(true)}>
            Remove album…
          </button>
        </div>
      </div>

      <ArtworkGallery
        images={album.covers}
        label="cover"
        onUpload={handleUploadCover}
        onSetPrimary={handleSetPrimaryCover}
        onDelete={handleDeleteCover}
      />

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
            <th></th>
            <th className="num">#</th>
            <th>Title</th>
            <th className="num">Duration</th>
          </tr>
        </thead>
        <tbody>
          {album.tracks.map((track, index) => (
            <tr key={track.id}>
              <td className="play">
                <PlayButton track={toPlayableTrack(track)} onPlay={() => handlePlay(index)} />
              </td>
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
