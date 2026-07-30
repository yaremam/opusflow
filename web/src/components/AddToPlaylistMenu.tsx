import { useEffect, useState } from 'react'
import {
  addTrackToPlaylist,
  createPlaylist,
  errorMessage,
  listPlaylists,
  listPlaylistsContainingTrack,
  type Playlist,
} from '../api/library'
import ModalScrim from './ModalScrim'
import './AddToPlaylistMenu.css'

interface AddToPlaylistMenuProps {
  trackId: number
  trackTitle: string
}

// AddToPlaylistMenu is the "⋯" overflow button every track row gets
// (AC-5) — a sibling of PlayButton/AddToQueueButton, not a 4th icon among
// them (backlog/028's grilling explicitly rejected that for crowding).
// Opens straight into the picker rather than an intermediate dropdown,
// since "Add to playlist" is the only action it exposes.
export default function AddToPlaylistMenu({ trackId, trackTitle }: AddToPlaylistMenuProps) {
  const [open, setOpen] = useState(false)

  function handleClick(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    setOpen(true)
  }

  return (
    <>
      <button type="button" className="add-to-playlist-btn" title="Add to playlist" onClick={handleClick}>
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <circle cx="5" cy="12" r="1.8" />
          <circle cx="12" cy="12" r="1.8" />
          <circle cx="19" cy="12" r="1.8" />
        </svg>
      </button>
      {open && <AddToPlaylistModal trackId={trackId} trackTitle={trackTitle} onClose={() => setOpen(false)} />}
    </>
  )
}

interface AddToPlaylistModalProps {
  trackId: number
  trackTitle: string
  onClose: () => void
}

function AddToPlaylistModal({ trackId, trackTitle, onClose }: AddToPlaylistModalProps) {
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [containingIds, setContainingIds] = useState<Set<number>>(new Set())
  const [loadError, setLoadError] = useState<string | null>(null)
  const [addingId, setAddingId] = useState<number | null>(null)
  const [addError, setAddError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [newName, setNewName] = useState('')
  const [createSubmitting, setCreateSubmitting] = useState(false)

  useEffect(() => {
    let cancelled = false
    Promise.all([listPlaylists({ pageSize: 500, sort: 'name' }).then((page) => page.items), listPlaylistsContainingTrack(trackId)])
      .then(([all, containing]) => {
        if (cancelled) return
        setPlaylists(all)
        setContainingIds(new Set(containing.map((p) => p.id)))
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
    // trackId is the only real dependency — this modal is remounted (not
    // re-rendered in place) whenever it's reopened for a different track.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trackId])

  async function handleAdd(playlist: Playlist) {
    setAddingId(playlist.id)
    setAddError(null)
    try {
      await addTrackToPlaylist(playlist.id, trackId)
      setContainingIds((prev) => new Set(prev).add(playlist.id))
    } catch (err) {
      setAddError(errorMessage(err))
    } finally {
      setAddingId(null)
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    if (!newName.trim()) return
    setCreateSubmitting(true)
    setAddError(null)
    try {
      const playlist = await createPlaylist(newName.trim())
      await addTrackToPlaylist(playlist.id, trackId)
      setPlaylists((prev) => [playlist, ...prev])
      setContainingIds((prev) => new Set(prev).add(playlist.id))
      setNewName('')
      setCreating(false)
    } catch (err) {
      setAddError(errorMessage(err))
    } finally {
      setCreateSubmitting(false)
    }
  }

  return (
    <ModalScrim label={`Add ${trackTitle} to playlist`} onClose={onClose} panelClassName="atp-panel">
      <h2>Add "{trackTitle}" to playlist</h2>

      {creating ? (
        <form className="atp-new-form" onSubmit={handleCreate}>
          <input
            type="text"
            placeholder="Playlist name"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            autoFocus
          />
          <div className="atp-new-actions">
            <button type="button" className="btn-ghost" onClick={() => setCreating(false)} disabled={createSubmitting}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={createSubmitting || !newName.trim()}>
              {createSubmitting ? 'Creating…' : 'Create & add'}
            </button>
          </div>
        </form>
      ) : (
        <button type="button" className="atp-new-row" onClick={() => setCreating(true)}>
          <span className="atp-new-plus">＋</span> New playlist…
        </button>
      )}

      {loadError && <p className="modal-error">{loadError}</p>}
      {addError && <p className="modal-error">{addError}</p>}

      {!loadError && playlists.length === 0 && !creating && <p className="sub">No playlists yet — create one above.</p>}

      <div className="atp-list">
        {playlists.map((playlist) => {
          const inPlaylist = containingIds.has(playlist.id)
          return (
            <button
              type="button"
              key={playlist.id}
              className="atp-row"
              disabled={inPlaylist || addingId === playlist.id}
              onClick={() => handleAdd(playlist)}
            >
              <span className={`atp-check${inPlaylist ? ' on' : ''}`}>{inPlaylist ? '✓' : ''}</span>
              <span className="atp-name">{playlist.name}</span>
            </button>
          )
        })}
      </div>

      <div className="modal-actions">
        <button type="button" className="btn-ghost modal-cancel" onClick={onClose}>
          Done
        </button>
      </div>
    </ModalScrim>
  )
}
