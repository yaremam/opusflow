import { Link } from 'react-router'
import { formatDuration, listSongs, type Song } from '../api/library'
import { useListPage } from '../hooks/useListPage'
import Pager from '../components/Pager'
import ArtTile from '../components/ArtTile'
import PlayButton from '../components/PlayButton'
import AddToQueueButton from '../components/AddToQueueButton'
import AddToPlaylistMenu from '../components/AddToPlaylistMenu'
import type { PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import '../styles/catalog.css'

function toPlayableTrack(song: Song): PlayableTrack {
  return {
    id: song.id,
    title: song.title,
    artistName: song.artistName,
    albumTitle: song.albumTitle,
    albumCoverThumbUrl: song.albumCoverThumbUrl,
    durationSeconds: song.durationSeconds,
    format: song.format,
  }
}

export default function SongsPage() {
  const { filters, setFilter, page, loadError, totalPages } = useListPage(listSongs)
  const player = usePlayer()

  function handlePlay(index: number) {
    if (!page) return
    player.playFrom(page.items.map(toPlayableTrack), index)
  }

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
        {page?.items.map((song, index) => (
          <Link key={song.id} className="song-row" to={`/albums/${song.albumId}`}>
            <PlayButton track={toPlayableTrack(song)} onPlay={() => handlePlay(index)} />
            <AddToQueueButton track={toPlayableTrack(song)} />
            <AddToPlaylistMenu trackId={song.id} trackTitle={song.title} />
            <ArtTile src={song.albumCoverThumbUrl} alt="" className="thumb" kind="album" artStatus={song.albumArtStatus} />
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
