import { Link } from 'react-router'
import { formatDuration, listSongs } from '../api/library'
import { useListPage } from '../hooks/useListPage'
import Pager from '../components/Pager'
import '../styles/catalog.css'

export default function SongsPage() {
  const { filters, setFilter, page, loadError, totalPages } = useListPage(listSongs)

  return (
    <div className="page-shell">
      <p className="eyebrow">{page ? `${page.totalCount} songs` : 'Songs'}</p>
      <div className="page-head">
        <h1>Songs</h1>
      </div>
      <p className="sub">Every song in your household library.</p>

      <div className="toolbar">
        <div className="search-wrap">
          <label htmlFor="song-q">Search</label>
          <input
            id="song-q"
            type="search"
            placeholder="Search songs, artists, or albums…"
            value={filters.q}
            onChange={(e) => setFilter('q', e.target.value)}
          />
        </div>
        <label>
          Sort
          <select value={filters.sort} onChange={(e) => setFilter('sort', e.target.value as 'recent' | 'name')}>
            <option value="recent">Recently added</option>
            <option value="name">Name (A–Z)</option>
          </select>
        </label>
        <label>
          Genre
          <input type="text" placeholder="Any genre" value={filters.genre} onChange={(e) => setFilter('genre', e.target.value)} />
        </label>
        <label>
          Year
          <input type="number" placeholder="Any year" value={filters.year} onChange={(e) => setFilter('year', e.target.value)} />
        </label>
      </div>

      {loadError && <p className="library-load-error">{loadError}</p>}

      {page && page.items.length === 0 && !loadError && <p className="sub">No songs match these filters.</p>}

      <div>
        {page?.items.map((song) => (
          <Link key={song.id} className="song-row" to={`/albums/${song.albumId}`}>
            <div>
              <div className="t">{song.title}</div>
              <div className="a">
                {song.artistName || 'Unknown Artist'} — {song.albumTitle || 'Unknown Album'}
              </div>
            </div>
            <div className="dur">{formatDuration(song.durationSeconds)}</div>
          </Link>
        ))}
      </div>

      <Pager page={filters.page} totalPages={totalPages} onChange={(p) => setFilter('page', p)} />
    </div>
  )
}
