// QueueState is the domain state machine backlog/019 AC-4 requires shared
// "across mobile/ and web/" — queue management, current-track state, and
// repeat/shuffle modes. It's generic over T (a platform's own track shape:
// web/src/player/context.ts's PlayableTrack, mobile/src/services/api.ts's
// Track) since neither platform's track fields matter to the queue math
// itself — only queue position and duration do, and duration stays with
// each platform's own currentTime tracking (see durationSeconds below).
//
// Deliberately excludes anything time-based (currentTime, seek position):
// web's PlayerContext splits that into its own TimeContext so a once-a-
// second <audio onTimeUpdate> tick doesn't re-render every queue consumer
// — folding currentTime into this state would undo that. Each platform
// adapter owns its own current-time state and decides what to do with it
// (e.g. a "restart the current track if more than a few seconds in"
// previous-track UX) before calling into prev()/next() here.
export type RepeatMode = 'off' | 'one' | 'all'

export interface QueueState<T> {
  queue: T[]
  currentIndex: number // -1 = nothing loaded
  isPlaying: boolean
  repeatMode: RepeatMode
  isShuffle: boolean
}

export function initialQueueState<T>(): QueueState<T> {
  return { queue: [], currentIndex: -1, isPlaying: false, repeatMode: 'off', isShuffle: false }
}

export function currentTrack<T>(state: QueueState<T>): T | null {
  return state.currentIndex >= 0 ? (state.queue[state.currentIndex] ?? null) : null
}

// playFrom replaces the queue with tracks starting at startIndex — the
// shared "click a track, queue the rest of the list it came from" behavior
// both platforms use.
export function playFrom<T>(state: QueueState<T>, tracks: T[], startIndex: number): QueueState<T> {
  const queue = tracks.slice(startIndex)
  if (queue.length === 0) return state
  return { ...state, queue, currentIndex: 0, isPlaying: true }
}

function withIndex<T>(state: QueueState<T>, index: number): QueueState<T> {
  return { ...state, currentIndex: index }
}

export function jumpTo<T>(state: QueueState<T>, index: number): QueueState<T> {
  return index >= 0 && index < state.queue.length ? withIndex(state, index) : state
}

// next also doubles as auto-advance (a track ending on its own): shuffle
// picks any track at random; otherwise it's the next queue slot, wrapping
// to the front only when repeatMode is 'all'; reaching the end with no
// wrap stops playback rather than looping silently.
export function next<T>(state: QueueState<T>): QueueState<T> {
  if (state.queue.length === 0) return state
  if (state.isShuffle) {
    return withIndex(state, Math.floor(Math.random() * state.queue.length))
  }
  if (state.currentIndex + 1 < state.queue.length) {
    return withIndex(state, state.currentIndex + 1)
  }
  if (state.repeatMode === 'all') {
    return withIndex(state, 0)
  }
  return { ...state, isPlaying: false }
}

// prev moves to the previous queue slot, or is a no-op at the front — a
// platform wanting "restart the current track instead, past a few seconds
// in" (mobile's existing behavior) makes that decision itself using its
// own current-time state before calling this.
export function prev<T>(state: QueueState<T>): QueueState<T> {
  return state.currentIndex > 0 ? withIndex(state, state.currentIndex - 1) : state
}

// indexAfterRemoval/indexAfterReorder are the queue-index arithmetic
// removeFromQueue/reorderQueue need — how the currently-playing index
// shifts when the queue in front of it changes.
export function indexAfterRemoval(currentIndex: number, removedIndex: number): number {
  return removedIndex < currentIndex ? currentIndex - 1 : currentIndex
}

export function indexAfterReorder(currentIndex: number, fromIndex: number, toIndex: number): number {
  if (fromIndex === currentIndex) return toIndex
  if (fromIndex < currentIndex && toIndex >= currentIndex) return currentIndex - 1
  if (fromIndex > currentIndex && toIndex <= currentIndex) return currentIndex + 1
  return currentIndex
}

// removeFromQueue leaves the now-playing track alone — removing it happens
// via skip (next/jumpTo), not this.
export function removeFromQueue<T>(state: QueueState<T>, index: number): QueueState<T> {
  if (index === state.currentIndex) return state
  const queue = state.queue.filter((_, i) => i !== index)
  return { ...state, queue, currentIndex: indexAfterRemoval(state.currentIndex, index) }
}

export function reorderQueue<T>(state: QueueState<T>, fromIndex: number, toIndex: number): QueueState<T> {
  if (fromIndex === toIndex) return state
  const queue = [...state.queue]
  const [moved] = queue.splice(fromIndex, 1)
  queue.splice(toIndex, 0, moved)
  return { ...state, queue, currentIndex: indexAfterReorder(state.currentIndex, fromIndex, toIndex) }
}

export function toggleShuffle<T>(state: QueueState<T>): QueueState<T> {
  return { ...state, isShuffle: !state.isShuffle }
}

const REPEAT_CYCLE: RepeatMode[] = ['off', 'all', 'one']

export function toggleRepeat<T>(state: QueueState<T>): QueueState<T> {
  const i = REPEAT_CYCLE.indexOf(state.repeatMode)
  return { ...state, repeatMode: REPEAT_CYCLE[(i + 1) % REPEAT_CYCLE.length] }
}

export function setPlaying<T>(state: QueueState<T>, isPlaying: boolean): QueueState<T> {
  return { ...state, isPlaying }
}
