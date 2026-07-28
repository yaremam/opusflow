import { useContext } from 'react'
import { TimeContext, type PlayerTimeState } from './context'

// usePlayerTime is split from usePlayer() so that only components that
// actually render a seek bar (MiniPlayer) re-render on every <audio
// onTimeUpdate> tick (many times a second during playback) — every other
// consumer of usePlayer() (song rows, pages) never subscribes to this.
export function usePlayerTime(): PlayerTimeState {
  return useContext(TimeContext)
}
