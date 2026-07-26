import { Link } from 'react-router'
import { listAlbums } from '../api/library'
import { useListPage } from '../hooks/useListPage'
import Pager from '../components/Pager'
import ArtTile from '../components/ArtTile'
import '../styles/catalog.css'

export default function AlbumsPage() {
  const { filters, setFilter, page, loadError, totalPages } = useListPage(listAlbums)

  return (
    <div className="page-shell wide">
      <p className="eyebrow">{page ? `${page.totalCount} albums` : 'Albums'}</p>
      <div className="page-head">
        <h1>Albums</h1>
      </div>
      <p className="sub">Every album in your household library.</p>

      <div className="toolbar">
        <div className="search-wrap">
          <label htmlFor="album-q">Search</label>
          <input
            id="album-q"
            type="search"
            placeholder="Search albums or artists…"
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

      {page && page.items.length === 0 && !loadError && <p className="sub">No albums match these filters.</p>}

      <div className="card-grid">
        {page?.items.map((album) => (
          <Link key={album.id} className="album-card" to={`/albums/${album.id}`}>
            <ArtTile src={album.coverThumbUrl} alt="" className="art" kind="album" />
            <div className="title">{album.title || 'Unknown Album'}</div>
            <div className="artist">
              {album.artistName || 'Unknown Artist'}
              {album.year > 0 ? ` · ${album.year}` : ''}
            </div>
          </Link>
        ))}
      </div>

      <Pager page={filters.page} totalPages={totalPages} onChange={(p) => setFilter('page', p)} />
    </div>
  )
}
