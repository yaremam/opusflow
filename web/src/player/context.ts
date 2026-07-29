import { createContext } from 'react'
import { initialQueueState, type QueueState, type RepeatMode } from '@opusflow/player-core'

export type { RepeatMode }

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

// PlayerState is web's adapter shape onto @opusflow/player-core's shared
// queue state machine (backlog/019 AC-4: queue management, current-track
// state, and repeat/shuffle modes, shared across web/ and mobile/ — see
// mobile/src/services/audioPlayer.ts for the other adapter). volume is
// appended locally since it's a playback-engine concern, not a queue one,
// the same reason current-time lives in TimeContext below rather than here.
export interface PlayerState extends QueueState<PlayableTrack> {
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
  toggleShuffle: () => void
  toggleRepeat: () => void
}

export const initialPlayerState: PlayerState = {
  ...initialQueueState<PlayableTrack>(),
  volume: 0.7,
}

export const initialPlayerTimeState: PlayerTimeState = {
  currentTime: 0,
  duration: 0,
}

export const PlayerContext = createContext<PlayerContextValue | null>(null)
export const TimeContext = createContext<PlayerTimeState>(initialPlayerTimeState)
