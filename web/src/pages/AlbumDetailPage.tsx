import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import {
  deleteAlbum,
  deleteAlbumCover,
  formatDuration,
  getAlbum,
  retryAlbumArt,
  setAlbumBannerCover,
  setAlbumPrimaryCover,
  uploadAlbumArt,
  type AlbumTrack,
} from '../api/library'
import ArtActions from '../components/ArtActions'
import ArtTile from '../components/ArtTile'
import ArtworkGallery from '../components/ArtworkGallery'
import InfoBlock from '../components/InfoBlock'
import PlayButton from '../components/PlayButton'
import RemoveModal from '../components/RemoveModal'
import { useEntityGallery, type EntityGalleryConfig } from '../hooks/useEntityGallery'
import type { PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import '../styles/catalog.css'

export default function AlbumDetailPage() {
  const { id } = useParams<{ id: string }>()
  const albumId = Number(id)
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

  const totalSeconds = album.tracks.reduce((sum, t) => sum + t.durationSeconds, 0)

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/albums">Albums</Link> / {album.title || 'Unknown Album'}
      </p>
      <div className="detail-banner-wrap">
        <ArtTile src={album.bannerUrl} alt="" className="detail-banner-img" kind="album" artStatus={album.artStatus} />
        <div className="detail-header-body">
          <ArtTile
            src={album.coverUrl || album.coverThumbUrl}
            alt=""
            className="detail-avatar"
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
            <button type="button" className="btn-ghost detail-remove" onClick={startRemove}>
              Remove album…
            </button>
          </div>
        </div>
      </div>

      <ArtworkGallery
        images={album.covers}
        label="cover"
        onUpload={handleUploadCover}
        onSetPrimary={handleSetPrimaryCover}
        onSetBanner={handleSetBannerCover}
        onDelete={handleDeleteCover}
      />

      {removing && (
        <RemoveModal
          name={album.title || 'Unknown Album'}
          submitting={removeSubmitting}
          submitError={removeError}
          onDeleteFiles={() => confirmRemove(true)}
          onKeepFiles={() => confirmRemove(false)}
          onCancel={cancelRemove}
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
