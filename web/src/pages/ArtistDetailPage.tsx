import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deleteArtist,
  deleteArtistPhoto,
  errorMessage,
  getArtist,
  pollForArtResolution,
  retryArtistArt,
  setArtistPrimaryPhoto,
  uploadArtistArt,
  type ArtistDetail,
} from '../api/library'
import ArtActions from '../components/ArtActions'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import InfoBlock from '../components/InfoBlock'
import RemoveModal from '../components/RemoveModal'
import '../styles/catalog.css'

export default function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [artist, setArtist] = useState<ArtistDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [removing, setRemoving] = useState(false)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)

  async function handleRetryArt() {
    if (!artist) return
    const queued = await retryArtistArt(artist.id)
    setArtist((prev) => prev && { ...prev, ...queued })
    const resolved = await pollForArtResolution(() => getArtist(artist.id))
    setArtist((prev) => prev && { ...prev, ...resolved })
  }

  async function handleUploadPhoto(file: File) {
    if (!artist) return
    const updated = await uploadArtistArt(artist.id, file)
    setArtist(updated)
  }

  async function handleSetPrimaryPhoto(photoId: number) {
    if (!artist) return
    const updated = await setArtistPrimaryPhoto(artist.id, photoId)
    setArtist(updated)
  }

  async function handleDeletePhoto(photoId: number, deleteFile: boolean) {
    if (!artist) return
    const updated = await deleteArtistPhoto(artist.id, photoId, deleteFile)
    setArtist(updated)
  }

  async function handleRemove(deleteFiles: boolean) {
    if (!artist) return
    setRemoveSubmitting(true)
    setRemoveError(null)
    try {
      await deleteArtist(artist.id, deleteFiles)
      navigate('/artists')
    } catch (err) {
      setRemoveError(errorMessage(err))
      setRemoveSubmitting(false)
    }
  }

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
        <ArtTile
          src={artist.photoUrl || artist.photoThumbUrl}
          alt=""
          className="detail-art round"
          kind="artist"
          artStatus={artist.artStatus}
        />
        <div className="detail-meta">
          <div className="kind">Artist</div>
          <h1>{artist.name || 'Unknown Artist'}</h1>
          <div className="facts">
            {artist.albumCount} album{artist.albumCount === 1 ? '' : 's'} · {artist.trackCount} song
            {artist.trackCount === 1 ? '' : 's'}
          </div>
          <ArtActions thumbUrl={artist.photoThumbUrl} artStatus={artist.artStatus} onRetry={handleRetryArt} />
          <button type="button" className="btn-ghost detail-remove" onClick={() => setRemoving(true)}>
            Remove artist…
          </button>
        </div>
      </div>

      <ArtworkGallery
        images={artist.photos}
        label="photo"
        onUpload={handleUploadPhoto}
        onSetPrimary={handleSetPrimaryPhoto}
        onDelete={handleDeletePhoto}
      />

      {removing && (
        <RemoveModal
          name={artist.name || 'Unknown Artist'}
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
          ...(artist.formedYear > 0 ? [{ label: 'Formed', value: String(artist.formedYear) }] : []),
          ...(artist.country ? [{ label: 'Country', value: artist.country }] : []),
          ...artist.genres.map((g) => ({ label: 'Genre', value: g })),
        ]}
        text={artist.bio}
        sourceUrl={artist.bioSourceUrl}
      />

      <div className="section-head">
        <h2>Albums</h2>
      </div>
      <div className="card-grid">
        {artist.albums.map((album) => (
          <Link key={album.id} className="album-card" to={`/albums/${album.id}`}>
            <ArtTile src={album.coverThumbUrl} alt="" className="art" kind="album" artStatus={album.artStatus} />
            <div className="title">{album.title || 'Unknown Album'}</div>
            <div className="artist">{album.year > 0 ? album.year : ''}</div>
          </Link>
        ))}
      </div>
    </div>
  )
}
