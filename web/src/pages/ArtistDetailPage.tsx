import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import { deleteArtist, deleteArtistPhoto, getArtist, retryArtistArt, setArtistPrimaryPhoto, uploadArtistArt } from '../api/library'
import ArtActions from '../components/ArtActions'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import InfoBlock from '../components/InfoBlock'
import RemoveModal from '../components/RemoveModal'
import { useEntityGallery, type EntityGalleryConfig } from '../hooks/useEntityGallery'
import '../styles/catalog.css'

export default function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const artistId = Number(id)
  const config = useMemo<EntityGalleryConfig<Awaited<ReturnType<typeof getArtist>>, Awaited<ReturnType<typeof retryArtistArt>>>>(
    () => ({
      fetchDetail: getArtist,
      retryArt: retryArtistArt,
      uploadImage: uploadArtistArt,
      setPrimaryImage: setArtistPrimaryPhoto,
      deleteImage: deleteArtistPhoto,
      deleteEntity: deleteArtist,
      afterRemove: '/artists',
    }),
    [],
  )
  const {
    entity: artist,
    loadError,
    retryArt: handleRetryArt,
    uploadImage: handleUploadPhoto,
    setPrimaryImage: handleSetPrimaryPhoto,
    deleteImage: handleDeletePhoto,
    removing,
    removeSubmitting,
    removeError,
    startRemove,
    cancelRemove,
    confirmRemove,
  } = useEntityGallery(artistId, config)

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
          <button type="button" className="btn-ghost detail-remove" onClick={startRemove}>
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
          onDeleteFiles={() => confirmRemove(true)}
          onKeepFiles={() => confirmRemove(false)}
          onCancel={cancelRemove}
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
