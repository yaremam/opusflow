import { isPlayable, type PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import './PlayButton.css'

interface PlayButtonProps {
  track: PlayableTrack
  onPlay: () => void
}

// PlayButton is the per-row play control shared by SongsPage and
// AlbumDetailPage's track table (TDR 015 AC-1). If this row is already
// the track playing, clicking toggles play/pause instead of restarting
// the queue from here; a WavPack track's button is disabled with an
// explanation, since no browser can decode that format (AC-6).
export default function PlayButton({ track, onPlay }: PlayButtonProps) {
  const player = usePlayer()
  const playable = isPlayable(track)
  const isCurrent = player.currentTrack?.id === track.id
  const showPause = isCurrent && player.isPlaying

  function handleClick(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    if (isCurrent) {
      player.togglePlayPause()
    } else {
      onPlay()
    }
  }

  return (
    <button
      type="button"
      className={`play-row-btn${showPause ? ' playing' : ''}`}
      disabled={!playable}
      title={playable ? (showPause ? 'Pause' : 'Play') : "WavPack can't play in-browser"}
      onClick={handleClick}
    >
      {showPause ? (
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M6 5h4v14H6zM14 5h4v14h-4z" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M8 5v14l11-7z" />
        </svg>
      )}
    </button>
  )
}
