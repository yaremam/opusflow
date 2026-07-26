import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import {
  errorMessage,
  formatDuration,
  listAlbums,
  listArtists,
  listImports,
  listSongs,
  type Album,
  type Artist,
  type Import,
  type Song,
} from '../api/library'
import ArtTile from '../components/ArtTile'
import '../styles/catalog.css'

const PREVIEW_COUNT = 8

// VIEW_MODE_KEY is the first bit of client-side UI state this app persists
// (TDR 008) — grid/table is a standing preference, not per-visit page
// state, so it's remembered across visits rather than resetting every load.
const VIEW_MODE_KEY = 'opusflow.homeViewMode'
type ViewMode = 'grid' | 'table'

function loadViewMode(): ViewMode {
  return localStorage.getItem(VIEW_MODE_KEY) === 'table' ? 'table' : 'grid'
}

function greeting(): string {
  const hour = new Date().getHours()
  if (hour < 12) return 'Good morning.'
  if (hour < 18) return 'Good afternoon.'
  return 'Good evening.'
}

interface HomeData {
  artists: Artist[]
  albums: Album[]
  songs: Song[]
  totalArtists: number
  totalAlbums: number
  totalSongs: number
  copyingImport: Import | null
}

export default function HomePage() {
  const navigate = useNavigate()
  const [data, setData] = useState<HomeData | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [viewMode, setViewMode] = useState<ViewMode>(loadViewMode)

  function setView(mode: ViewMode) {
    setViewMode(mode)
    localStorage.setItem(VIEW_MODE_KEY, mode)
  }

  useEffect(() => {
    let cancelled = false
    Promise.all([
      listArtists({ sort: 'recent', pageSize: PREVIEW_COUNT }),
      listAlbums({ sort: 'recent', pageSize: PREVIEW_COUNT }),
      listSongs({ sort: 'recent', pageSize: PREVIEW_COUNT }),
      listImports(),
    ])
      .then(([artists, albums, songs, imports]) => {
        if (cancelled) return
        setData({
          artists: artists.items,
          albums: albums.items,
          songs: songs.items,
          totalArtists: artists.totalCount,
          totalAlbums: albums.totalCount,
          totalSongs: songs.totalCount,
          copyingImport: imports.find((imp) => imp.status === 'copying') ?? null,
        })
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(errorMessage(err))
      })
    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="page-shell">
      <p className="eyebrow">Household library</p>
      <div className="page-head">
        <h1>{greeting()}</h1>
        {data && data.totalSongs > 0 && (
          <Link className="btn-primary" to="/import">
            ＋ Import music
          </Link>
        )}
      </div>
      <p className="sub">Here's what's in your shared library right now.</p>

      {loadError && <p className="library-load-error">{loadError}</p>}

      {data && data.totalSongs === 0 && (
        <div className="empty-hero">
          <div className="glyph">
            <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
              <path
                d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinejoin="round"
              />
            </svg>
          </div>
          <h1>No music yet</h1>
          <p>Import your first album from a mounted folder or straight from this device — opusflow copies and organizes it automatically.</p>
          <Link className="btn-primary" to="/import">
            ＋ Import your first album
          </Link>
        </div>
      )}

      {data && data.totalSongs > 0 && (
        <>
          <div className="stat-grid">
            <div className="stat-card">
              <div className="n">{data.totalArtists}</div>
              <div className="label">Artists</div>
            </div>
            <div className="stat-card">
              <div className="n">{data.totalAlbums}</div>
              <div className="label">Albums</div>
            </div>
            <div className="stat-card">
              <div className="n">{data.totalSongs}</div>
              <div className="label">Songs</div>
            </div>
          </div>

          {data.copyingImport && (
            <div className="scan-banner">
              <span className="pulse" />
              Importing from "{data.copyingImport.sourceDescription}" — {data.copyingImport.filesProcessed} of ~
              {data.copyingImport.filesTotal || '?'} files copied.
            </div>
          )}

          <div className="toolbar-row">
            <span className="hint">Recently added, newest first</span>
            <div className="view-toggle">
              <button type="button" className={viewMode === 'grid' ? 'active' : ''} onClick={() => setView('grid')}>
                ▦ Grid
              </button>
              <button type="button" className={viewMode === 'table' ? 'active' : ''} onClick={() => setView('table')}>
                ☰ Table
              </button>
            </div>
          </div>

          <div className="section-head">
            <h2>Recently added artists</h2>
            <Link to="/artists">See all artists →</Link>
          </div>
          {viewMode === 'grid' ? (
            <div className="chip-row">
              {data.artists.map((artist) => (
                <Link key={artist.id} className="artist-chip" to={`/artists/${artist.id}`}>
                  <ArtTile src={artist.photoThumbUrl} alt="" className="avatar" kind="artist" artStatus={artist.artStatus} />
                  <span className="name">{artist.name || 'Unknown Artist'}</span>
                </Link>
              ))}
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Artist</th>
                  <th className="num">Albums</th>
                  <th className="num">Songs</th>
                </tr>
              </thead>
              <tbody>
                {data.artists.map((artist) => (
                  <tr key={artist.id} className="row-link" onClick={() => navigate(`/artists/${artist.id}`)}>
                    <td>
                      <div className="row-thumb">
                        <ArtTile
                          src={artist.photoThumbUrl}
                          alt=""
                          className="avatar sm"
                          kind="artist"
                          artStatus={artist.artStatus}
                        />
                        <span className="name">{artist.name || 'Unknown Artist'}</span>
                      </div>
                    </td>
                    <td className="num">{artist.albumCount}</td>
                    <td className="num">{artist.trackCount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <div className="section-head">
            <h2>Recently added albums</h2>
            <Link to="/albums">See all albums →</Link>
          </div>
          {viewMode === 'grid' ? (
            <div className="card-grid">
              {data.albums.map((album) => (
                <Link key={album.id} className="album-card" to={`/albums/${album.id}`}>
                  <ArtTile src={album.coverThumbUrl} alt="" className="art" kind="album" artStatus={album.artStatus} />
                  <div className="title">{album.title || 'Unknown Album'}</div>
                  <div className="artist">{album.artistName || 'Unknown Artist'}</div>
                </Link>
              ))}
            </div>
          ) : (
            <table className="data-table">
              <thead>
                <tr>
                  <th>Album</th>
                  <th>Artist</th>
                  <th className="num">Year</th>
                </tr>
              </thead>
              <tbody>
                {data.albums.map((album) => (
                  <tr key={album.id} className="row-link" onClick={() => navigate(`/albums/${album.id}`)}>
                    <td>
                      <div className="row-thumb">
                        <ArtTile src={album.coverThumbUrl} alt="" className="art sm" kind="album" artStatus={album.artStatus} />
                        <span className="name">{album.title || 'Unknown Album'}</span>
                      </div>
                    </td>
                    <td>{album.artistName || 'Unknown Artist'}</td>
                    <td className="num">{album.year > 0 ? album.year : ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          <div className="section-head">
            <h2>Recently added songs</h2>
            <Link to="/songs">See all songs →</Link>
          </div>
          <div>
            {data.songs.map((song) => (
              <Link key={song.id} className="song-row" to={`/albums/${song.albumId}`}>
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
        </>
      )}
    </div>
  )
}
