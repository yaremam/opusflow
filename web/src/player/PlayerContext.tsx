import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { streamURL } from '../api/library'
import {
  PlayerContext,
  TimeContext,
  indexAfterRemoval,
  indexAfterReorder,
  initialPlayerState,
  initialPlayerTimeState,
  isPlayable,
  type PlayableTrack,
  type PlayerContextValue,
  type PlayerState,
} from './context'

// withIndex is the one state shape next/prev/jumpTo all produce — moving
// the "current track" pointer without touching anything else. next also
// doubles as the <audio onEnded> handler (auto-advance): reaching the end
// of the queue is a no-op here, and playback naturally stops on its own
// once the last track's audio element pauses itself at end-of-media.
function withIndex(state: PlayerState, index: number): PlayerState {
  return { ...state, currentIndex: index }
}

export function PlayerProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PlayerState>(initialPlayerState)
  const [time, setTime] = useState(initialPlayerTimeState)
  const audioRef = useRef<HTMLAudioElement>(null)

  const currentTrack = state.currentIndex >= 0 ? (state.queue[state.currentIndex] ?? null) : null

  // Load whichever track currentIndex points at into the shared <audio>
  // element and start playing it — runs whenever the track identity
  // changes, not on every state update, so seeking/volume changes don't
  // reload (and restart) the source.
  const currentTrackId = currentTrack?.id
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || currentTrackId === undefined) return
    audio.src = streamURL(currentTrackId)
    setTime(initialPlayerTimeState)
    audio.play().catch(() => {
      // Autoplay can be blocked before any user gesture; the play button
      // click that got us here counts as one, but a defensive catch keeps
      // a rejected promise from surfacing as an unhandled error. Playing
      // vs. not is reflected by the audio element's own play/pause events
      // regardless (see the <audio> handlers below), not set here.
    })
  }, [currentTrackId])

  useEffect(() => {
    const audio = audioRef.current
    if (audio) audio.volume = state.volume
  }, [state.volume])

  const playFrom = useCallback((tracks: PlayableTrack[], startIndex: number) => {
    const playable = tracks.slice(startIndex).filter(isPlayable)
    if (playable.length === 0) return
    setState((prev) => ({ ...prev, queue: playable, currentIndex: 0 }))
  }, [])

  // togglePlayPause and advance are the only places that call the audio
  // element's play()/pause() directly — isPlaying itself is always set
  // from the element's own 'play'/'pause' events (below), one source of
  // truth instead of every action hand-syncing a duplicate flag.
  const togglePlayPause = useCallback(() => {
    const audio = audioRef.current
    if (!audio) return
    if (audio.paused) {
      audio.play().catch(() => {})
    } else {
      audio.pause()
    }
  }, [])

  const seek = useCallback((seekTime: number) => {
    const audio = audioRef.current
    if (!audio) return
    audio.currentTime = seekTime
    setTime((prev) => ({ ...prev, currentTime: seekTime }))
  }, [])

  const jumpTo = useCallback((index: number) => {
    setState((prev) => (index >= 0 && index < prev.queue.length ? withIndex(prev, index) : prev))
  }, [])

  const next = useCallback(() => {
    setState((prev) => (prev.currentIndex + 1 < prev.queue.length ? withIndex(prev, prev.currentIndex + 1) : prev))
  }, [])

  const prev = useCallback(() => {
    setState((p) => (p.currentIndex > 0 ? withIndex(p, p.currentIndex - 1) : p))
  }, [])

  const removeFromQueue = useCallback((index: number) => {
    setState((prev) => {
      if (index === prev.currentIndex) return prev // removing the now-playing track happens via skip, not this
      const queue = prev.queue.filter((_, i) => i !== index)
      return { ...prev, queue, currentIndex: indexAfterRemoval(prev.currentIndex, index) }
    })
  }, [])

  const reorderQueue = useCallback((fromIndex: number, toIndex: number) => {
    setState((prev) => {
      if (fromIndex === toIndex) return prev
      const queue = [...prev.queue]
      const [moved] = queue.splice(fromIndex, 1)
      queue.splice(toIndex, 0, moved)
      return { ...prev, queue, currentIndex: indexAfterReorder(prev.currentIndex, fromIndex, toIndex) }
    })
  }, [])

  const setVolume = useCallback((volume: number) => {
    setState((prev) => ({ ...prev, volume: Math.min(1, Math.max(0, volume)) }))
  }, [])

  const value = useMemo<PlayerContextValue>(
    () => ({
      ...state,
      currentTrack,
      playFrom,
      togglePlayPause,
      seek,
      next,
      prev,
      removeFromQueue,
      reorderQueue,
      jumpTo,
      setVolume,
    }),
    [state, currentTrack, playFrom, togglePlayPause, seek, next, prev, removeFromQueue, reorderQueue, jumpTo, setVolume],
  )

  return (
    <PlayerContext.Provider value={value}>
      <TimeContext.Provider value={time}>
        {children}
        <audio
          ref={audioRef}
          onPlay={() => setState((prev) => ({ ...prev, isPlaying: true }))}
          onPause={() => setState((prev) => ({ ...prev, isPlaying: false }))}
          onTimeUpdate={(e) => {
            const currentTime = e.currentTarget.currentTime
            setTime((prev) => ({ ...prev, currentTime }))
          }}
          onLoadedMetadata={(e) => {
            const duration = e.currentTarget.duration
            setTime((prev) => ({ ...prev, duration }))
          }}
          onEnded={next}
          hidden
        />
      </TimeContext.Provider>
    </PlayerContext.Provider>
  )
}
