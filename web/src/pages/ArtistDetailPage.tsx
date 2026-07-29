import { useMemo } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deleteArtist,
  deleteArtistPhoto,
  getArtist,
  listArtists,
  mergeArtist,
  retryArtistArt,
  setArtistBannerPhoto,
  setArtistPrimaryPhoto,
  uploadArtistArt,
} from '../api/library'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import EntityDetailHeader from '../components/EntityDetailHeader'
import InfoBlock from '../components/InfoBlock'
import { useEntityGallery, type EntityGalleryConfig } from '../hooks/useEntityGallery'
import '../styles/catalog.css'

export default function ArtistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const artistId = Number(id)
  const navigate = useNavigate()
  const config = useMemo<EntityGalleryConfig<Awaited<ReturnType<typeof getArtist>>, Awaited<ReturnType<typeof retryArtistArt>>>>(
    () => ({
      fetchDetail: getArtist,
      retryArt: retryArtistArt,
      uploadImage: uploadArtistArt,
      setPrimaryImage: setArtistPrimaryPhoto,
      setBannerImage: setArtistBannerPhoto,
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
    setBannerImage: handleSetBannerPhoto,
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

      <EntityDetailHeader
        kind="artist"
        displayName={artistDisplayName}
        bannerSrc={artist.bannerUrl}
        avatarSrc={artist.photoUrl || artist.photoThumbUrl}
        facts={
          <>
            {artist.albumCount} album{artist.albumCount === 1 ? '' : 's'} · {artist.trackCount} song
            {artist.trackCount === 1 ? '' : 's'}
          </>
        }
        thumbUrl={artist.photoThumbUrl}
        artStatus={artist.artStatus}
        onRetryArt={handleRetryArt}
        remove={{
          removing,
          submitting: removeSubmitting,
          error: removeError,
          start: startRemove,
          cancel: cancelRemove,
          confirm: confirmRemove,
        }}
        merge={{
          sourceSub: `${artist.albumCount} album${artist.albumCount === 1 ? '' : 's'} · ${artist.trackCount} song${artist.trackCount === 1 ? '' : 's'}`,
          effects: [
            `Move ${artist.albumCount} album${artist.albumCount === 1 ? '' : 's'} and ${artist.trackCount} song${artist.trackCount === 1 ? '' : 's'} from "${artistDisplayName}" onto the artist you pick — combining any same-titled album instead of duplicating it.`,
            `Move "${artistDisplayName}"'s photos into the kept artist's gallery.`,
            `Leave every audio file exactly where it is on disk — only the catalog entries change.`,
          ],
          search: async (q) => {
            const page = await listArtists({ q, pageSize: 20 })
            return page.items
              .filter((a) => a.id !== artistId)
              .map((a) => ({
                id: a.id,
                name: a.name || 'Unknown Artist',
                sub: `${a.albumCount} album${a.albumCount === 1 ? '' : 's'} · ${a.trackCount} song${a.trackCount === 1 ? '' : 's'}`,
              }))
          },
          merge: (intoId) => mergeArtist(artistId, intoId),
          onMerged: (intoId) => navigate(`/artists/${intoId}`),
        }}
      />

      <ArtworkGallery
        images={artist.photos}
        label="photo"
        onUpload={handleUploadPhoto}
        onSetPrimary={handleSetPrimaryPhoto}
        onSetBanner={handleSetBannerPhoto}
        onDelete={handleDeletePhoto}
      />

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
