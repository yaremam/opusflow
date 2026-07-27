import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { streamURL } from '../api/library'
import { PlayerContext, initialPlayerState, isPlayable, type PlayableTrack, type PlayerContextValue, type PlayerState } from './context'

export function PlayerProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<PlayerState>(initialPlayerState)
  const audioRef = useRef<HTMLAudioElement>(null)

  const currentTrack = state.currentIndex >= 0 ? (state.queue[state.currentIndex] ?? null) : null

  // Load whichever track currentIndex points at into the shared <audio>
  // element and start playing it — runs whenever the track identity
  // changes, not on every state update, so seeking/volume changes don't
  // reload (and restart) the source.
  useEffect(() => {
    const audio = audioRef.current
    if (!audio || !currentTrack) return
    audio.src = streamURL(currentTrack.id)
    audio.play().catch(() => {
      // Autoplay can be blocked before any user gesture; the play button
      // click that got us here counts as one, but a defensive catch keeps
      // a rejected promise from surfacing as an unhandled error.
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentTrack?.id])

  useEffect(() => {
    const audio = audioRef.current
    if (audio) audio.volume = state.volume
  }, [state.volume])

  const advance = useCallback(() => {
    setState((prev) => {
      if (prev.currentIndex + 1 < prev.queue.length) {
        return { ...prev, currentIndex: prev.currentIndex + 1, currentTime: 0, isPlaying: true }
      }
      return { ...prev, isPlaying: false, currentTime: 0 }
    })
  }, [])

  const playFrom = useCallback((tracks: PlayableTrack[], startIndex: number) => {
    const playable = tracks.slice(startIndex).filter(isPlayable)
    if (playable.length === 0) return
    setState((prev) => ({ ...prev, queue: playable, currentIndex: 0, isPlaying: true, currentTime: 0, duration: 0 }))
  }, [])

  const pause = useCallback(() => {
    audioRef.current?.pause()
    setState((prev) => ({ ...prev, isPlaying: false }))
  }, [])

  const resume = useCallback(() => {
    audioRef.current?.play().catch(() => {})
    setState((prev) => ({ ...prev, isPlaying: true }))
  }, [])

  const togglePlayPause = useCallback(() => {
    setState((prev) => {
      if (prev.currentIndex < 0) return prev
      if (prev.isPlaying) {
        audioRef.current?.pause()
      } else {
        audioRef.current?.play().catch(() => {})
      }
      return { ...prev, isPlaying: !prev.isPlaying }
    })
  }, [])

  const seek = useCallback((time: number) => {
    const audio = audioRef.current
    if (!audio) return
    audio.currentTime = time
    setState((prev) => ({ ...prev, currentTime: time }))
  }, [])

  const next = useCallback(() => {
    setState((prev) => {
      if (prev.currentIndex + 1 >= prev.queue.length) return prev
      return { ...prev, currentIndex: prev.currentIndex + 1, currentTime: 0, isPlaying: true }
    })
  }, [])

  const prev = useCallback(() => {
    setState((prevState) => {
      if (prevState.currentIndex <= 0) return prevState
      return { ...prevState, currentIndex: prevState.currentIndex - 1, currentTime: 0, isPlaying: true }
    })
  }, [])

  const removeFromQueue = useCallback((index: number) => {
    setState((prev) => {
      if (index === prev.currentIndex) return prev // removing the now-playing track happens via skip, not this
      const queue = prev.queue.filter((_, i) => i !== index)
      const currentIndex = index < prev.currentIndex ? prev.currentIndex - 1 : prev.currentIndex
      return { ...prev, queue, currentIndex }
    })
  }, [])

  const reorderQueue = useCallback((fromIndex: number, toIndex: number) => {
    setState((prev) => {
      if (fromIndex === toIndex) return prev
      const queue = [...prev.queue]
      const [moved] = queue.splice(fromIndex, 1)
      queue.splice(toIndex, 0, moved)

      let currentIndex = prev.currentIndex
      if (fromIndex === prev.currentIndex) {
        currentIndex = toIndex
      } else if (fromIndex < prev.currentIndex && toIndex >= prev.currentIndex) {
        currentIndex -= 1
      } else if (fromIndex > prev.currentIndex && toIndex <= prev.currentIndex) {
        currentIndex += 1
      }
      return { ...prev, queue, currentIndex }
    })
  }, [])

  const jumpTo = useCallback((index: number) => {
    setState((prev) => {
      if (index < 0 || index >= prev.queue.length) return prev
      return { ...prev, currentIndex: index, currentTime: 0, isPlaying: true }
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
      pause,
      resume,
      togglePlayPause,
      seek,
      next,
      prev,
      removeFromQueue,
      reorderQueue,
      jumpTo,
      setVolume,
    }),
    [state, currentTrack, playFrom, pause, resume, togglePlayPause, seek, next, prev, removeFromQueue, reorderQueue, jumpTo, setVolume],
  )

  return (
    <PlayerContext.Provider value={value}>
      {children}
      <audio
        ref={audioRef}
        onTimeUpdate={(e) => setState((prev) => ({ ...prev, currentTime: e.currentTarget.currentTime }))}
        onLoadedMetadata={(e) => setState((prev) => ({ ...prev, duration: e.currentTarget.duration }))}
        onEnded={advance}
        hidden
      />
    </PlayerContext.Provider>
  )
}
