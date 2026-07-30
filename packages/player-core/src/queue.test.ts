import { describe, expect, it } from 'vitest'
import {
  addToQueue,
  currentTrack,
  indexAfterReorder,
  indexAfterRemoval,
  initialQueueState,
  jumpTo,
  next,
  playFrom,
  prev,
  removeFromQueue,
  reorderQueue,
  toggleRepeat,
  toggleShuffle,
  type QueueState,
} from './queue'

interface T {
  id: number
}

function track(id: number): T {
  return { id }
}

function queueOf(ids: number[], currentIndex: number): QueueState<T> {
  return { queue: ids.map(track), currentIndex, isPlaying: true, repeatMode: 'off', isShuffle: false }
}

describe('playFrom', () => {
  it('starts a fresh queue from startIndex', () => {
    const state = playFrom(initialQueueState<T>(), [track(1), track(2), track(3)], 1)
    expect(state.queue.map((t) => t.id)).toEqual([2, 3])
    expect(state.currentIndex).toBe(0)
    expect(state.isPlaying).toBe(true)
  })

  it('is a no-op when startIndex leaves nothing to play', () => {
    const initial = initialQueueState<T>()
    expect(playFrom(initial, [track(1)], 5)).toBe(initial)
  })
})

describe('addToQueue', () => {
  it('appends to the end without touching the currently playing track (AC-1)', () => {
    const state = queueOf([1, 2, 3], 1)
    const next = addToQueue(state, track(4))
    expect(next.queue.map((t) => t.id)).toEqual([1, 2, 3, 4])
    expect(next.currentIndex).toBe(1)
    expect(next.isPlaying).toBe(true)
  })

  it('starts playing immediately when the queue was empty (AC-2)', () => {
    const next = addToQueue(initialQueueState<T>(), track(1))
    expect(next.queue.map((t) => t.id)).toEqual([1])
    expect(next.currentIndex).toBe(0)
    expect(next.isPlaying).toBe(true)
  })

  it('allows a duplicate — no dedup against tracks already queued (AC-3)', () => {
    const state = queueOf([1, 2], 0)
    const next = addToQueue(state, track(2))
    expect(next.queue.map((t) => t.id)).toEqual([1, 2, 2])
  })
})

describe('next', () => {
  it('advances to the next track', () => {
    const state = next(queueOf([1, 2, 3], 0))
    expect(state.currentIndex).toBe(1)
  })

  it('stops playback at the end with no repeat', () => {
    const state = next(queueOf([1, 2, 3], 2))
    expect(state.currentIndex).toBe(2)
    expect(state.isPlaying).toBe(false)
  })

  it('wraps to the front when repeatMode is all', () => {
    const state = next({ ...queueOf([1, 2, 3], 2), repeatMode: 'all' })
    expect(state.currentIndex).toBe(0)
  })

  it('picks a random track when shuffled', () => {
    const state = next({ ...queueOf([1, 2, 3], 0), isShuffle: true })
    expect(state.currentIndex).toBeGreaterThanOrEqual(0)
    expect(state.currentIndex).toBeLessThan(3)
  })
})

describe('prev', () => {
  it('moves to the previous track', () => {
    expect(prev(queueOf([1, 2, 3], 2)).currentIndex).toBe(1)
  })

  it('is a no-op at the front of the queue', () => {
    const state = queueOf([1, 2, 3], 0)
    expect(prev(state)).toBe(state)
  })
})

describe('jumpTo', () => {
  it('jumps to a valid index', () => {
    expect(jumpTo(queueOf([1, 2, 3], 0), 2).currentIndex).toBe(2)
  })

  it('ignores an out-of-range index', () => {
    const state = queueOf([1, 2, 3], 0)
    expect(jumpTo(state, 9)).toBe(state)
    expect(jumpTo(state, -1)).toBe(state)
  })
})

describe('indexAfterRemoval', () => {
  it('shifts left when a track before the current one is removed', () => {
    expect(indexAfterRemoval(3, 1)).toBe(2)
  })
  it('holds when a track after the current one is removed', () => {
    expect(indexAfterRemoval(1, 3)).toBe(1)
  })
})

describe('removeFromQueue', () => {
  it('removes a track and shifts the current index', () => {
    const state = removeFromQueue(queueOf([1, 2, 3], 2), 0)
    expect(state.queue.map((t) => t.id)).toEqual([2, 3])
    expect(state.currentIndex).toBe(1)
  })

  it('leaves the now-playing track alone', () => {
    const state = queueOf([1, 2, 3], 1)
    expect(removeFromQueue(state, 1)).toBe(state)
  })
})

describe('indexAfterReorder', () => {
  it('follows the current track when it is the one moved', () => {
    expect(indexAfterReorder(2, 2, 0)).toBe(0)
  })
  it('shifts left when something before the current track moves past it', () => {
    expect(indexAfterReorder(2, 0, 3)).toBe(1)
  })
  it('shifts right when something after the current track moves in front of it', () => {
    expect(indexAfterReorder(2, 4, 0)).toBe(3)
  })
  it('holds when the move does not cross the current track', () => {
    expect(indexAfterReorder(2, 3, 4)).toBe(2)
  })
})

describe('reorderQueue', () => {
  it('moves a track and updates the current index', () => {
    const state = reorderQueue(queueOf([1, 2, 3], 0), 0, 2)
    expect(state.queue.map((t) => t.id)).toEqual([2, 3, 1])
    expect(state.currentIndex).toBe(2)
  })
})

describe('toggleShuffle / toggleRepeat', () => {
  it('flips isShuffle', () => {
    const state = queueOf([1], 0)
    expect(toggleShuffle(state).isShuffle).toBe(true)
  })

  it('cycles off -> all -> one -> off', () => {
    let state = queueOf([1], 0)
    state = toggleRepeat(state)
    expect(state.repeatMode).toBe('all')
    state = toggleRepeat(state)
    expect(state.repeatMode).toBe('one')
    state = toggleRepeat(state)
    expect(state.repeatMode).toBe('off')
  })
})

describe('currentTrack', () => {
  it('reads the queue slot at currentIndex', () => {
    expect(currentTrack(queueOf([1, 2, 3], 1))?.id).toBe(2)
  })
  it('is null when nothing is loaded', () => {
    expect(currentTrack(initialQueueState<T>())).toBeNull()
  })
})
