import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import {
  addToQueue as coreAddToQueue,
  jumpTo as coreJumpTo,
  next as coreNext,
  playFrom as corePlayFrom,
  prev as corePrev,
  removeFromQueue as coreRemoveFromQueue,
  reorderQueue as coreReorderQueue,
  toggleRepeat as coreToggleRepeat,
  toggleShuffle as coreToggleShuffle,
} from '@opusflow/player-core'
import { streamURL } from '../api/library'
import {
  initialPlayerState,
  initialPlayerTimeState,
  isPlayable,
  PlayerContext,
  TimeContext,
  type PlayableTrack,
  type PlayerContextValue,
  type PlayerState,
} from './context'

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

  // playFrom/next/prev/jumpTo delegate their queue-index arithmetic to
  // @opusflow/player-core's shared reducer (backlog/019 AC-4) — but always
  // restore isPlaying to whatever it already was afterward. isPlaying is
  // never set from these actions directly here; it's always set from the
  // <audio> element's own 'play'/'pause' events below, one source of truth
  // instead of every action hand-syncing a duplicate flag. The shared core
  // doesn't know that convention (it sets isPlaying itself on playFrom/at
  // a stopped queue boundary), so each call restores it.
  const playFrom = useCallback((tracks: PlayableTrack[], startIndex: number) => {
    setState((prev) => {
      const playable = tracks.slice(startIndex).filter(isPlayable)
      if (playable.length === 0) return prev
      return { ...prev, ...corePlayFrom(prev, playable, 0), isPlaying: prev.isPlaying }
    })
  }, [])

  // addToQueue (backlog/025) appends without disturbing current playback
  // — same isPlaying-restoration convention as every other action here:
  // the <audio> element's own play/pause events are the one source of
  // truth, not what the shared core's reducer sets. When the queue was
  // empty, coreAddToQueue makes the added track current, which flows
  // through the currentTrackId effect above to actually load and play it.
  const addToQueue = useCallback((track: PlayableTrack) => {
    if (!isPlayable(track)) return
    setState((prev) => ({ ...prev, ...coreAddToQueue(prev, track), isPlaying: prev.isPlaying }))
  }, [])

  // togglePlayPause and advance are the only places that call the audio
  // element's play()/pause() directly.
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
    setState((prev) => ({ ...prev, ...coreJumpTo(prev, index) }))
  }, [])

  const next = useCallback(() => {
    setState((prev) => ({ ...prev, ...coreNext(prev), isPlaying: prev.isPlaying }))
  }, [])

  const prev = useCallback(() => {
    setState((p) => ({ ...p, ...corePrev(p), isPlaying: p.isPlaying }))
  }, [])

  const removeFromQueue = useCallback((index: number) => {
    setState((prev) => ({ ...prev, ...coreRemoveFromQueue(prev, index) }))
  }, [])

  const reorderQueue = useCallback((fromIndex: number, toIndex: number) => {
    setState((prev) => ({ ...prev, ...coreReorderQueue(prev, fromIndex, toIndex) }))
  }, [])

  const setVolume = useCallback((volume: number) => {
    setState((prev) => ({ ...prev, volume: Math.min(1, Math.max(0, volume)) }))
  }, [])

  const toggleShuffle = useCallback(() => {
    setState((prev) => ({ ...prev, ...coreToggleShuffle(prev) }))
  }, [])

  const toggleRepeat = useCallback(() => {
    setState((prev) => ({ ...prev, ...coreToggleRepeat(prev) }))
  }, [])

  const value = useMemo<PlayerContextValue>(
    () => ({
      ...state,
      currentTrack,
      playFrom,
      addToQueue,
      togglePlayPause,
      seek,
      next,
      prev,
      removeFromQueue,
      reorderQueue,
      jumpTo,
      setVolume,
      toggleShuffle,
      toggleRepeat,
    }),
    [state, currentTrack, playFrom, addToQueue, togglePlayPause, seek, next, prev, removeFromQueue, reorderQueue, jumpTo, setVolume, toggleShuffle, toggleRepeat],
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
