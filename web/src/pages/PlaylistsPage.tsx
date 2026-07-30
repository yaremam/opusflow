import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { createPlaylist, errorMessage, listPlaylists, type Playlist } from '../api/library'
import ModalScrim from '../components/ModalScrim'
import PlaylistCoverTile from '../components/PlaylistCoverTile'
import '../styles/catalog.css'
import '../styles/playlists.css'

export default function PlaylistsPage() {
  const navigate = useNavigate()
  const [playlists, setPlaylists] = useState<Playlist[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [q, setQ] = useState('')
  const [sort, setSort] = useState<'recent' | 'name'>('recent')

  const [creating, setCreating] = useState(false)
  const [createName, setCreateName] = useState('')
  const [createSubmitting, setCreateSubmitting] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    listPlaylists({ q: q || undefined, sort, pageSize: 200 })
      .then((page) => {
        if (!cancelled) setPlaylists(page.items)
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [q, sort])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    const name = createName.trim()
    if (!name) return
    setCreateSubmitting(true)
    setCreateError(null)
    try {
      const playlist = await createPlaylist(name)
      navigate(`/playlists/${playlist.id}`)
    } catch (err) {
      setCreateError(errorMessage(err))
      setCreateSubmitting(false)
    }
  }

  return (
    <div className="page-shell wide">
      <p className="eyebrow">{playlists ? `${playlists.length} playlist${playlists.length === 1 ? '' : 's'}` : 'Playlists'}</p>
      <div className="page-head">
        <h1>Playlists</h1>
      </div>
      <p className="sub">Household collections, in whatever order you want them.</p>

      <div className="toolbar">
        <div className="search-wrap">
          <label htmlFor="playlist-q">Search</label>
          <input
            id="playlist-q"
            type="search"
            placeholder="Search playlists…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
        </div>
        <label>
          Sort
          <select value={sort} onChange={(e) => setSort(e.target.value as 'recent' | 'name')}>
            <option value="recent">Recently added</option>
            <option value="name">Name (A–Z)</option>
          </select>
        </label>
      </div>

      {loadError && <p className="library-load-error">{loadError}</p>}

      <div className="card-grid">
        <button type="button" className="playlist-card new-playlist-card" onClick={() => setCreating(true)}>
          <span className="new-playlist-glyph">＋</span>
          <span className="title">New playlist</span>
        </button>
        {playlists?.map((playlist) => (
          <Link key={playlist.id} className="playlist-card" to={`/playlists/${playlist.id}`}>
            <PlaylistCoverTile coverUrls={playlist.coverUrls} className="cover" />
            <div className="title">{playlist.name}</div>
            <div className="sub-line">
              {playlist.trackCount} song{playlist.trackCount === 1 ? '' : 's'}
            </div>
          </Link>
        ))}
      </div>

      {creating && (
        <ModalScrim label="Create a playlist" onClose={() => !createSubmitting && setCreating(false)}>
          <h2>New playlist</h2>
          <form onSubmit={handleCreate}>
            <input
              type="text"
              className="playlist-name-input"
              placeholder="Playlist name"
              value={createName}
              onChange={(e) => setCreateName(e.target.value)}
              autoFocus
            />
            {createError && <p className="modal-error">{createError}</p>}
            <div className="modal-actions">
              <button type="submit" className="btn-primary" disabled={createSubmitting || !createName.trim()}>
                {createSubmitting ? 'Creating…' : 'Create playlist'}
              </button>
              <button
                type="button"
                className="btn-ghost modal-cancel"
                disabled={createSubmitting}
                onClick={() => setCreating(false)}
              >
                Cancel
              </button>
            </div>
          </form>
        </ModalScrim>
      )}
    </div>
  )
}
