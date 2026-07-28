import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deleteArtist,
  deleteArtistPhoto,
  getArtist,
  listArtists,
  mergeArtist,
  retryArtistArt,
  setArtistPrimaryPhoto,
  uploadArtistArt,
} from '../api/library'
import ArtActions from '../components/ArtActions'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import InfoBlock from '../components/InfoBlock'
import MergeModal from '../components/MergeModal'
import RemoveModal from '../components/RemoveModal'
import { useEntityGallery, type EntityGalleryConfig } from '../hooks/useEntityGallery'
import '../styles/catalog.css'

export default function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const artistId = Number(id)
  const navigate = useNavigate()
  const [merging, setMerging] = useState(false)
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

  const artistDisplayName = artist.name || 'Unknown Artist'

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/artists">Artists</Link> / {artistDisplayName}
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
          <h1>{artistDisplayName}</h1>
          <div className="facts">
            {artist.albumCount} album{artist.albumCount === 1 ? '' : 's'} · {artist.trackCount} song
            {artist.trackCount === 1 ? '' : 's'}
          </div>
          <ArtActions thumbUrl={artist.photoThumbUrl} artStatus={artist.artStatus} onRetry={handleRetryArt} />
          <div className="detail-secondary-actions">
            <button type="button" className="btn-ghost" onClick={() => setMerging(true)}>
              ⇄ Merge into…
            </button>
            <button type="button" className="btn-ghost detail-remove" onClick={startRemove}>
              Remove artist…
            </button>
          </div>
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
          name={artistDisplayName}
          submitting={removeSubmitting}
          submitError={removeError}
          onDeleteFiles={() => confirmRemove(true)}
          onKeepFiles={() => confirmRemove(false)}
          onCancel={cancelRemove}
        />
      )}

      {merging && (
        <MergeModal
          label="artist"
          sourceName={artistDisplayName}
          sourceSub={`${artist.albumCount} album${artist.albumCount === 1 ? '' : 's'} · ${artist.trackCount} song${artist.trackCount === 1 ? '' : 's'}`}
          effects={[
            `Move ${artist.albumCount} album${artist.albumCount === 1 ? '' : 's'} and ${artist.trackCount} song${artist.trackCount === 1 ? '' : 's'} from "${artistDisplayName}" onto the artist you pick — combining any same-titled album instead of duplicating it.`,
            `Move "${artistDisplayName}"'s photos into the kept artist's gallery.`,
            `Leave every audio file exactly where it is on disk — only the catalog entries change.`,
          ]}
          search={async (q) => {
            const page = await listArtists({ q, pageSize: 20 })
            return page.items
              .filter((a) => a.id !== artistId)
              .map((a) => ({
                id: a.id,
                name: a.name || 'Unknown Artist',
                sub: `${a.albumCount} album${a.albumCount === 1 ? '' : 's'} · ${a.trackCount} song${a.trackCount === 1 ? '' : 's'}`,
              }))
          }}
          merge={(intoId) => mergeArtist(artistId, intoId)}
          onClose={() => setMerging(false)}
          onMerged={(intoId) => {
            setMerging(false)
            navigate(`/artists/${intoId}`)
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
