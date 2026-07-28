import { createContext } from 'react'

// PlayableTrack is the shape the player needs to display and stream a
// track — deliberately its own type rather than reusing Song/AlbumTrack
// directly, since AlbumDetailPage's rows don't repeat the album/artist
// name per row (the page itself already provides that context) while the
// mini-player always needs it regardless of which page started playback.
// Callers (SongsPage, AlbumDetailPage) map their own rows into this shape.
export interface PlayableTrack {
  id: number
  title: string
  artistName: string
  albumTitle: string
  albumCoverThumbUrl: string
  durationSeconds: number
  format: string
}

// PLAYABLE_FORMATS is an allowlist of every format this app imports
// (backend/internal/library/scan/format.go's durationParsersByExt) that a
// browser's <audio> element can actually decode — every one except
// WavPack (TDR 015 AC-6, the one format no browser supports at all). An
// allowlist fails closed for any future format this app starts importing
// but the player doesn't handle yet, rather than silently assuming it's
// playable.
const PLAYABLE_FORMATS = new Set(['mp3', 'flac', 'm4a', 'aac', 'ogg', 'wav'])

export function isPlayable(track: PlayableTrack): boolean {
  return PLAYABLE_FORMATS.has(track.format)
}

export interface PlayerState {
  queue: PlayableTrack[]
  currentIndex: number // -1 = nothing loaded
  isPlaying: boolean
  volume: number
}

// PlayerTimeState is split out of PlayerState (and given its own context,
// see TimeContext below) because currentTime changes many times a second
// during playback — keeping it out of the main state/context value means
// a <audio onTimeUpdate> tick only re-renders whatever actually reads
// PlayerTimeState (the mini-player's seek bar), not every consumer of
// usePlayer() (every song row's PlayButton, every page that just wants
// the stable playFrom action).
export interface PlayerTimeState {
  currentTime: number
  duration: number
}

export interface PlayerContextValue extends PlayerState {
  currentTrack: PlayableTrack | null
  playFrom: (tracks: PlayableTrack[], startIndex: number) => void
  togglePlayPause: () => void
  seek: (time: number) => void
  next: () => void
  prev: () => void
  removeFromQueue: (index: number) => void
  reorderQueue: (fromIndex: number, toIndex: number) => void
  jumpTo: (index: number) => void
  setVolume: (volume: number) => void
}

export const initialPlayerState: PlayerState = {
  queue: [],
  currentIndex: -1,
  isPlaying: false,
  volume: 0.7,
}

export const initialPlayerTimeState: PlayerTimeState = {
  currentTime: 0,
  duration: 0,
}

export const PlayerContext = createContext<PlayerContextValue | null>(null)
export const TimeContext = createContext<PlayerTimeState>(initialPlayerTimeState)

// indexAfterRemoval/indexAfterReorder are the queue-index arithmetic
// removeFromQueue/reorderQueue need — pulled out as pure functions rather
// than inline conditionals in a setState updater, since "how does the
// currently-playing index shift when the queue in front of it changes"
// is subtle enough to want to read (and eventually test) on its own.
export function indexAfterRemoval(currentIndex: number, removedIndex: number): number {
  return removedIndex < currentIndex ? currentIndex - 1 : currentIndex
}

export function indexAfterReorder(currentIndex: number, fromIndex: number, toIndex: number): number {
  if (fromIndex === currentIndex) return toIndex
  if (fromIndex < currentIndex && toIndex >= currentIndex) return currentIndex - 1
  if (fromIndex > currentIndex && toIndex <= currentIndex) return currentIndex + 1
  return currentIndex
}
