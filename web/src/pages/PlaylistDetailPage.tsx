import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  deletePlaylist,
  errorMessage,
  formatDuration,
  getPlaylist,
  removePlaylistTrack,
  renamePlaylist,
  reorderPlaylistTracks,
  type PlaylistDetail,
  type PlaylistTrack,
} from '../api/library'
import AddToPlaylistMenu from '../components/AddToPlaylistMenu'
import AddToQueueButton from '../components/AddToQueueButton'
import ModalScrim from '../components/ModalScrim'
import PlayButton from '../components/PlayButton'
import PlaylistCoverTile from '../components/PlaylistCoverTile'
import type { PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import '../styles/catalog.css'
import '../styles/playlists.css'

function toPlayableTrack(track: PlaylistTrack): PlayableTrack {
  return {
    id: track.trackId,
    title: track.title,
    artistName: track.artistName,
    albumTitle: track.albumTitle,
    albumCoverThumbUrl: track.albumCoverThumbUrl,
    durationSeconds: track.durationSeconds,
    format: track.format,
  }
}

export default function PlaylistDetailPage() {
  const { id } = useParams<{ id: string }>()
  const playlistId = Number(id)
  const navigate = useNavigate()
  const player = usePlayer()

  const [detail, setDetail] = useState<PlaylistDetail | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [rowError, setRowError] = useState<string | null>(null)

  const [renaming, setRenaming] = useState(false)
  const [renameValue, setRenameValue] = useState('')
  const [renameSubmitting, setRenameSubmitting] = useState(false)

  const [deleting, setDeleting] = useState(false)
  const [deleteSubmitting, setDeleteSubmitting] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false
    getPlaylist(playlistId)
      .then((d) => {
        if (!cancelled) setDetail(d)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [playlistId])

  function handlePlay(index: number) {
    if (!detail) return
    player.playFrom(detail.tracks.map(toPlayableTrack), index)
  }

  function startRename() {
    if (!detail) return
    setRenameValue(detail.name)
    setRenaming(true)
  }

  async function commitRename() {
    if (!detail) return
    const trimmed = renameValue.trim()
    if (!trimmed || trimmed === detail.name) {
      setRenaming(false)
      return
    }
    setRenameSubmitting(true)
    try {
      const updated = await renamePlaylist(playlistId, trimmed)
      setDetail(updated)
      setRenaming(false)
    } catch (err) {
      setRowError(errorMessage(err))
    } finally {
      setRenameSubmitting(false)
    }
  }

  async function confirmDelete() {
    setDeleteSubmitting(true)
    setDeleteError(null)
    try {
      await deletePlaylist(playlistId)
      navigate('/playlists')
    } catch (err) {
      setDeleteError(errorMessage(err))
      setDeleteSubmitting(false)
    }
  }

  async function handleRemoveTrack(playlistTrackId: number) {
    setRowError(null)
    try {
      setDetail(await removePlaylistTrack(playlistId, playlistTrackId))
    } catch (err) {
      setRowError(errorMessage(err))
    }
  }

  async function handleDrop(toIndex: number) {
    const fromIndex = dragIndex
    setDragIndex(null)
    setDragOverIndex(null)
    if (fromIndex === null || fromIndex === toIndex || !detail) return
    const track = detail.tracks[fromIndex]
    setRowError(null)
    try {
      const updated = await reorderPlaylistTracks(playlistId, track.playlistTrackId, toIndex)
      setDetail(updated)
    } catch (err) {
      setRowError(errorMessage(err))
    }
  }

  if (loadError) {
    return (
      <div className="page-shell">
        <p className="crumb">
          <Link to="/playlists">Playlists</Link>
        </p>
        <p className="library-load-error">{loadError}</p>
      </div>
    )
  }

  if (!detail) return null

  const totalSeconds = detail.tracks.reduce((sum, t) => sum + t.durationSeconds, 0)

  return (
    <div className="page-shell">
      <p className="crumb">
        <Link to="/playlists">Playlists</Link> / {detail.name}
      </p>

      <div className="playlist-hero">
        <PlaylistCoverTile coverUrls={detail.coverUrls} className="playlist-hero-cover" />
        <div className="playlist-hero-meta">
          {renaming ? (
            <input
              className="playlist-rename-input"
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              onBlur={commitRename}
              onKeyDown={(e) => {
                if (e.key === 'Enter') commitRename()
                if (e.key === 'Escape') setRenaming(false)
              }}
              disabled={renameSubmitting}
              autoFocus
            />
          ) : (
            <h1 className="playlist-title-editable" title="Click to rename" onClick={startRename}>
              {detail.name}
            </h1>
          )}
          <p className="sub">
            {detail.trackCount} song{detail.trackCount === 1 ? '' : 's'}
            {totalSeconds > 0 ? ` · ${formatDuration(totalSeconds)}` : ''}
          </p>
          <div className="detail-secondary-actions">
            <button type="button" className="btn-ghost detail-remove" onClick={() => setDeleting(true)}>
              Delete playlist…
            </button>
          </div>
        </div>
      </div>

      {rowError && <p className="library-load-error">{rowError}</p>}

      {detail.tracks.length === 0 ? (
        <p className="sub" style={{ marginTop: '2rem' }}>
          No songs yet — add some from Songs or an album's track list.
        </p>
      ) : (
        <table className="track-table playlist-track-table">
          <thead>
            <tr>
              <th></th>
              <th></th>
              <th></th>
              <th></th>
              <th>Title</th>
              <th className="num">Duration</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {detail.tracks.map((track, index) => (
              <tr
                key={track.playlistTrackId}
                className={`${dragIndex === index ? 'dragging' : ''} ${
                  dragOverIndex === index && dragIndex !== null && dragIndex !== index ? 'drag-over' : ''
                }`}
                draggable
                onDragStart={() => setDragIndex(index)}
                onDragEnd={() => {
                  setDragIndex(null)
                  setDragOverIndex(null)
                }}
                onDragOver={(e) => {
                  if (dragIndex === null) return
                  e.preventDefault()
                  setDragOverIndex(index)
                }}
                onDragLeave={() => setDragOverIndex((prev) => (prev === index ? null : prev))}
                onDrop={(e) => {
                  e.preventDefault()
                  handleDrop(index)
                }}
              >
                <td className="drag">⠿</td>
                <td className="play">
                  <PlayButton track={toPlayableTrack(track)} onPlay={() => handlePlay(index)} />
                </td>
                <td className="queue">
                  <AddToQueueButton track={toPlayableTrack(track)} />
                </td>
                <td className="menu">
                  <AddToPlaylistMenu trackId={track.trackId} trackTitle={track.title} />
                </td>
                <td className="t">{track.title}</td>
                <td className="dur">{formatDuration(track.durationSeconds)}</td>
                <td className="remove">
                  <button
                    type="button"
                    className="btn-icon"
                    title="Remove from playlist"
                    aria-label={`Remove ${track.title} from playlist`}
                    onClick={() => handleRemoveTrack(track.playlistTrackId)}
                  >
                    ✕
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {deleting && (
        <ModalScrim label={`Delete ${detail.name}`} onClose={() => !deleteSubmitting && setDeleting(false)}>
          <h2>Delete "{detail.name}"?</h2>
          <p>This removes the playlist and its track order. The songs themselves stay in your library.</p>
          {deleteError && <p className="modal-error">{deleteError}</p>}
          <div className="modal-actions">
            <button type="button" className="btn-bad" disabled={deleteSubmitting} onClick={confirmDelete}>
              {deleteSubmitting ? 'Deleting…' : 'Delete playlist'}
            </button>
            <button
              type="button"
              className="btn-ghost modal-cancel"
              disabled={deleteSubmitting}
              onClick={() => setDeleting(false)}
            >
              Cancel
            </button>
          </div>
        </ModalScrim>
      )}
    </div>
  )
}
