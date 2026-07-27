import { useState } from 'react'
import { usePlayer } from '../player/usePlayer'
import ArtTile from './ArtTile'
import './QueueDrawer.css'

interface QueueDrawerProps {
  open: boolean
}

// QueueDrawer shows every upcoming track in the queue (TDR 015 AC-5) —
// the current track first (marked, not draggable/removable), then every
// track after it. Reordering uses native HTML5 drag-and-drop; this app
// has no existing drag-and-drop library and the interaction is simple
// enough for the platform primitives alone.
export default function QueueDrawer({ open }: QueueDrawerProps) {
  const player = usePlayer()
  const { queue, currentIndex } = player
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null)

  if (!open) return null

  const upcoming = queue.map((track, index) => ({ track, index })).slice(currentIndex >= 0 ? currentIndex : 0)
  const upcomingCount = Math.max(0, upcoming.length - 1)

  function handleDrop(toIndex: number) {
    if (dragIndex !== null && dragIndex !== toIndex) {
      player.reorderQueue(dragIndex, toIndex)
    }
    setDragIndex(null)
    setDragOverIndex(null)
  }

  return (
    <div className="queue-drawer">
      <div className="queue-drawer-head">
        <h3>Queue</h3>
        <span className="count">
          {upcomingCount} up next
        </span>
      </div>
      <div className="queue-list">
        {upcoming.length === 0 ? (
          <div className="queue-empty">Nothing queued — press play on a song.</div>
        ) : (
          upcoming.map(({ track, index }) => {
            const isCurrent = index === currentIndex
            return (
              <div
                key={`${track.id}-${index}`}
                className={`queue-row${dragIndex === index ? ' dragging' : ''}${dragOverIndex === index && dragIndex !== null && dragIndex !== index ? ' drag-over' : ''}`}
                draggable={!isCurrent}
                onDragStart={() => setDragIndex(index)}
                onDragEnd={() => {
                  setDragIndex(null)
                  setDragOverIndex(null)
                }}
                onDragOver={(e) => {
                  if (isCurrent || dragIndex === null) return
                  e.preventDefault()
                  setDragOverIndex(index)
                }}
                onDragLeave={() => setDragOverIndex((prev) => (prev === index ? null : prev))}
                onDrop={(e) => {
                  e.preventDefault()
                  if (!isCurrent) handleDrop(index)
                }}
                onClick={() => {
                  if (!isCurrent) player.jumpTo(index)
                }}
              >
                <span className="handle">
                  {isCurrent ? (
                    '▶'
                  ) : (
                    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                      <path d="M9 6a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm0 6a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm6-6a2 2 0 1 1 0 4 2 2 0 0 1 0-4zm0 6a2 2 0 1 1 0 4 2 2 0 0 1 0-4z" />
                    </svg>
                  )}
                </span>
                <ArtTile src={track.albumCoverThumbUrl} alt="" className="art" kind="album" />
                <div className="meta">
                  <div className="title">{track.title}</div>
                  <div className="artist">{track.artistName || 'Unknown Artist'}</div>
                </div>
                {!isCurrent && (
                  <button
                    type="button"
                    className="remove"
                    title="Remove from queue"
                    onClick={(e) => {
                      e.stopPropagation()
                      player.removeFromQueue(index)
                    }}
                  >
                    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                      <path d="M18.3 5.71 12 12l6.3 6.29-1.41 1.42L10.59 13.4 4.3 19.7 2.89 18.3 9.17 12 2.89 5.71 4.3 4.3l6.29 6.3 6.29-6.3z" />
                    </svg>
                  </button>
                )}
              </div>
            )
          })
        )}
      </div>
    </div>
  )
}
