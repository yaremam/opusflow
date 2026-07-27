import { usePlayer } from '../player/usePlayer'
import ArtTile from './ArtTile'
import './MiniPlayer.css'

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00'
  const m = Math.floor(seconds / 60)
  const s = Math.floor(seconds % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

interface MiniPlayerProps {
  queueOpen: boolean
  onToggleQueue: () => void
}

// MiniPlayer is the persistent bottom-docked player bar (TDR 015) — mounted
// once in AppLayout alongside the routed page's <Outlet />, so it survives
// navigating between pages while a track keeps playing. Renders nothing
// when no track has been played yet.
export default function MiniPlayer({ queueOpen, onToggleQueue }: MiniPlayerProps) {
  const player = usePlayer()
  const { currentTrack, isPlaying, currentTime, duration, volume, currentIndex, queue } = player

  if (!currentTrack) return null

  function handleSeekClick(e: React.MouseEvent<HTMLDivElement>) {
    if (!duration) return
    const rect = e.currentTarget.getBoundingClientRect()
    const pct = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    player.seek(pct * duration)
  }

  function handleVolumeClick(e: React.MouseEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    const pct = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width))
    player.setVolume(pct)
  }

  const seekPct = duration > 0 ? (currentTime / duration) * 100 : 0

  return (
    <div className="mini-player">
      <div className="mini-player-inner">
        <div className="now-playing-info">
          <ArtTile src={currentTrack.albumCoverThumbUrl} alt="" className="art" kind="album" />
          <div className="meta">
            <div className="title">{currentTrack.title}</div>
            <div className="artist">{currentTrack.artistName || 'Unknown Artist'}</div>
          </div>
        </div>

        <div className="transport">
          <div className="transport-buttons">
            <button type="button" onClick={player.prev} disabled={currentIndex <= 0} title="Previous">
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M6 6h2v12H6zm3.5 6 8.5 6V6z" />
              </svg>
            </button>
            <button type="button" className="play-pause" onClick={player.togglePlayPause} title={isPlaying ? 'Pause' : 'Play'}>
              {isPlaying ? (
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M6 5h4v14H6zM14 5h4v14h-4z" />
                </svg>
              ) : (
                <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                  <path d="M8 5v14l11-7z" />
                </svg>
              )}
            </button>
            <button type="button" onClick={player.next} disabled={currentIndex >= queue.length - 1} title="Next">
              <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M16 6h2v12h-2zM6 6l8.5 6L6 18z" />
              </svg>
            </button>
          </div>
          <div className="seek-row">
            <span className="seek-time">{formatTime(currentTime)}</span>
            <div className="seek-track" onClick={handleSeekClick}>
              <div className="seek-fill" style={{ width: `${seekPct}%` }} />
            </div>
            <span className="seek-time end">{formatTime(duration)}</span>
          </div>
        </div>

        <div className="right-controls">
          <div className="volume-row">
            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M3 10v4h4l5 5V5L7 10H3zm13.5 2A4.5 4.5 0 0 0 15 8.2v7.6a4.5 4.5 0 0 0 1.5-3.8z" />
            </svg>
            <div className="volume-track" onClick={handleVolumeClick}>
              <div className="volume-fill" style={{ width: `${volume * 100}%` }} />
            </div>
          </div>
          <button type="button" className={`queue-toggle${queueOpen ? ' open' : ''}`} onClick={onToggleQueue}>
            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M4 6h16v2H4zm0 5h16v2H4zm0 5h10v2H4z" />
            </svg>
            Queue
          </button>
        </div>
      </div>
    </div>
  )
}
