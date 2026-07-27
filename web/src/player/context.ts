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

// WAVPACK_FORMAT is the one format (TDR 015 AC-6) no browser's <audio>
// element can decode — filtered out of every queue before it's ever set,
// so the player itself never has to special-case "can't play this."
const WAVPACK_FORMAT = 'wv'

export function isPlayable(track: PlayableTrack): boolean {
  return track.format !== WAVPACK_FORMAT
}

export interface PlayerState {
  queue: PlayableTrack[]
  currentIndex: number // -1 = nothing loaded
  isPlaying: boolean
  currentTime: number
  duration: number
  volume: number
}

export interface PlayerContextValue extends PlayerState {
  currentTrack: PlayableTrack | null
  playFrom: (tracks: PlayableTrack[], startIndex: number) => void
  pause: () => void
  resume: () => void
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
  currentTime: 0,
  duration: 0,
  volume: 0.7,
}

export const PlayerContext = createContext<PlayerContextValue | null>(null)
