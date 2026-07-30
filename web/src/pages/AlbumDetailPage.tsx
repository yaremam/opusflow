import { useMemo } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deleteAlbum,
  deleteAlbumCover,
  formatDuration,
  getAlbum,
  listAlbums,
  mergeAlbum,
  retryAlbumArt,
  setAlbumBannerCover,
  setAlbumPrimaryCover,
  uploadAlbumArt,
  type AlbumTrack,
} from '../api/library'
import ArtworkGallery from '../components/ArtworkGallery'
import EntityDetailHeader from '../components/EntityDetailHeader'
import InfoBlock from '../components/InfoBlock'
import PlayButton from '../components/PlayButton'
import AddToQueueButton from '../components/AddToQueueButton'
import AddToPlaylistMenu from '../components/AddToPlaylistMenu'
import { useEntityGallery, type EntityGalleryConfig } from '../hooks/useEntityGallery'
import type { PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import '../styles/catalog.css'

export default function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const albumId = Number(id)
  const navigate = useNavigate()
  const player = usePlayer()
  const config = useMemo<EntityGalleryConfig<Awaited<ReturnType<typeof getAlbum>>, Awaited<ReturnType<typeof retryAlbumArt>>>>(
    () => ({
      fetchDetail: getAlbum,
      retryArt: retryAlbumArt,
      uploadImage: uploadAlbumArt,
      setPrimaryImage: setAlbumPrimaryCover,
      setBannerImage: setAlbumBannerCover,
      deleteImage: deleteAlbumCover,
      deleteEntity: deleteAlbum,
      afterRemove: '/albums',
    }),
    [],
  )
  const {
    entity: album,
    loadError,
    retryArt: handleRetryArt,
    uploadImage: handleUploadCover,
    setPrimaryImage: handleSetPrimaryCover,
    setBannerImage: handleSetBannerCover,
    deleteImage: handleDeleteCover,
    removing,
    removeSubmitting,
    removeError,
    startRemove,
    cancelRemove,
    confirmRemove,
  } = useEntityGallery(albumId, config)

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

  const albumDisplayName = album.title || 'Unknown Album'
  const totalSeconds = album.tracks.reduce((sum, t) => sum + t.durationSeconds, 0)

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/albums">Albums</Link> / {albumDisplayName}
      </p>

      <EntityDetailHeader
        kind="album"
        displayName={albumDisplayName}
        bannerSrc={album.bannerUrl}
        avatarSrc={album.coverUrl || album.coverThumbUrl}
        byLine={
          <div className="by">
            by <Link to={`/artists/${album.artistId}`}>{album.artistName || 'Unknown Artist'}</Link>
          </div>
        }
        facts={
          <>
            {album.year > 0 ? `${album.year} · ` : ''}
            {album.trackCount} song{album.trackCount === 1 ? '' : 's'} · {formatDuration(totalSeconds)}
          </>
        }
        thumbUrl={album.coverThumbUrl}
        artStatus={album.artStatus}
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
          sourceSub: `${album.trackCount} song${album.trackCount === 1 ? '' : 's'}`,
          effects: [
            `Move ${album.trackCount} song${album.trackCount === 1 ? '' : 's'} from "${albumDisplayName}" onto the album you pick.`,
            `Move "${albumDisplayName}"'s cover images into the kept album's gallery.`,
            `Leave every audio file exactly where it is on disk — only the catalog entries change.`,
          ],
          search: async (q) => {
            const page = await listAlbums({ q, pageSize: 20 })
            return page.items
              .filter((a) => a.id !== albumId && a.artistId === album.artistId)
              .map((a) => ({
                id: a.id,
                name: a.title || 'Unknown Album',
                sub: `${a.trackCount} song${a.trackCount === 1 ? '' : 's'}`,
              }))
          },
          merge: (intoId) => mergeAlbum(albumId, intoId),
          onMerged: (intoId) => navigate(`/albums/${intoId}`),
        }}
      />

      <ArtworkGallery
        images={album.covers}
        label="cover"
        onUpload={handleUploadCover}
        onSetPrimary={handleSetPrimaryCover}
        onSetBanner={handleSetBannerCover}
        onDelete={handleDeleteCover}
      />

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
            <th></th>
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
              <td className="queue">
                <AddToQueueButton track={toPlayableTrack(track)} />
              </td>
              <td className="menu">
                <AddToPlaylistMenu trackId={track.id} trackTitle={track.title} />
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
