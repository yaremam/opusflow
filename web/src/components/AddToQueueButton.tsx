import { useState } from 'react'
import { isPlayable, type PlayableTrack } from '../player/context'
import { usePlayer } from '../player/usePlayer'
import './AddToQueueButton.css'

interface AddToQueueButtonProps {
  track: PlayableTrack
}

// AddToQueueButton is the per-row "add to queue" control shared by
// SongsPage and AlbumDetailPage's track table (backlog/025 AC-4) —
// PlayButton's sibling, not a variant of it: this never disturbs
// whatever's currently playing, it only appends. Briefly shows a
// checkmark on click (AC-5) since appending has no other visible effect
// on this row.
export default function AddToQueueButton({ track }: AddToQueueButtonProps) {
  const player = usePlayer()
  const playable = isPlayable(track)
  const [justAdded, setJustAdded] = useState(false)

  function handleClick(e: React.MouseEvent) {
    e.preventDefault()
    e.stopPropagation()
    player.addToQueue(track)
    setJustAdded(true)
    setTimeout(() => setJustAdded(false), 1500)
  }

  return (
    <button
      type="button"
      className="queue-row-btn"
      disabled={!playable}
      title={playable ? 'Add to queue' : "WavPack can't play in-browser"}
      onClick={handleClick}
    >
      {justAdded ? (
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M9 16.2 4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4z" />
        </svg>
      ) : (
        <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 5v14M5 12h14" stroke="currentColor" strokeWidth="2" fill="none" strokeLinecap="round" />
        </svg>
      )}
    </button>
  )
}
