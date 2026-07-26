import { Link } from 'react-router'
import { listArtists } from '../api/library'
import { useListPage } from '../hooks/useListPage'
import Pager from '../components/Pager'
import ArtTile from '../components/ArtTile'
import '../styles/catalog.css'

export default function ArtistsPage() {
  const { filters, setFilter, page, loadError, totalPages } = useListPage(listArtists)

  return (
    <div className="page-shell wide">
      <p className="eyebrow">{page ? `${page.totalCount} artists` : 'Artists'}</p>
      <div className="page-head">
        <h1>Artists</h1>
      </div>
      <p className="sub">Every artist in your household library.</p>

      <div className="toolbar">
        <div className="search-wrap">
          <label htmlFor="artist-q">Search</label>
          <input
            id="artist-q"
            type="search"
            placeholder="Search artists…"
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

      {page && page.items.length === 0 && !loadError && <p className="sub">No artists match these filters.</p>}

      <div className="card-grid">
        {page?.items.map((artist) => (
          <Link key={artist.id} className="artist-card" to={`/artists/${artist.id}`}>
            <ArtTile src={artist.photoThumbUrl} alt="" className="avatar" kind="artist" />
            <div className="name">{artist.name || 'Unknown Artist'}</div>
            <div className="meta">
              {artist.albumCount} album{artist.albumCount === 1 ? '' : 's'}
            </div>
          </Link>
        ))}
      </div>

      <Pager page={filters.page} totalPages={totalPages} onChange={(p) => setFilter('page', p)} />
    </div>
  )
}
