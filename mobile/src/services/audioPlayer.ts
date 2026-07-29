import {
  currentTrack as coreCurrentTrack,
  initialQueueState,
  next as coreNext,
  playFrom as corePlayFrom,
  prev as corePrev,
  toggleRepeat as coreToggleRepeat,
  toggleShuffle as coreToggleShuffle,
  type QueueState,
  type RepeatMode,
} from '@opusflow/player-core';
import { Track } from './api';

export type { RepeatMode };

export interface AudioPlayerState {
  currentTrack: Track | null;
  queue: Track[];
  queueIndex: number;
  isPlaying: boolean;
  currentTimeSeconds: number;
  durationSeconds: number;
  volume: number;
  repeatMode: RepeatMode;
  isShuffle: boolean;
}

type StateChangeListener = (state: AudioPlayerState) => void;

// AudioPlayerEngine is this platform's adapter onto @opusflow/player-core's
// shared queue state machine (backlog/019 AC-4) — it owns exactly the two
// things that are genuinely platform-specific: current playback time (kept
// out of the shared core the same way web/src/player's TimeContext keeps it
// out of PlayerContext, so a once-a-second tick doesn't ripple through
// queue state) and volume. Everything about *which* track is playing next
// — advance, repeat, shuffle — is the shared reducer; this class is just
// wiring plus the "restart the current track rather than skip back, once
// more than a few seconds in" previous-track UX, which needs currentTime
// and so can't live in the shared core either.
class AudioPlayerEngine {
  private core: QueueState<Track> = initialQueueState<Track>();
  private currentTimeSeconds = 0;
  private volume = 1.0;

  private listeners: Set<StateChangeListener> = new Set();

  public getState(): AudioPlayerState {
    const track = coreCurrentTrack(this.core);
    return {
      currentTrack: track,
      queue: this.core.queue,
      queueIndex: this.core.currentIndex,
      isPlaying: this.core.isPlaying,
      currentTimeSeconds: this.currentTimeSeconds,
      durationSeconds: track?.durationSeconds ?? 0,
      volume: this.volume,
      repeatMode: this.core.repeatMode,
      isShuffle: this.core.isShuffle,
    };
  }

  public subscribe(listener: StateChangeListener): () => void {
    this.listeners.add(listener);
    listener(this.getState());
    return () => this.listeners.delete(listener);
  }

  private notify() {
    const currentState = this.getState();
    this.listeners.forEach((listener) => listener(currentState));
  }

  public playQueue(tracks: Track[], startIndex: number = 0) {
    const next = corePlayFrom(this.core, tracks, startIndex);
    if (next === this.core) return;
    this.core = next;
    this.currentTimeSeconds = 0;
    this.notify();
  }

  public togglePlayPause() {
    if (!coreCurrentTrack(this.core) && this.core.queue.length > 0) {
      this.playQueue(this.core.queue, 0);
      return;
    }
    this.core = { ...this.core, isPlaying: !this.core.isPlaying };
    this.notify();
  }

  public nextTrack() {
    if (this.core.queue.length === 0) return;
    const stopping = !this.core.isShuffle && this.core.currentIndex >= this.core.queue.length - 1 && this.core.repeatMode !== 'all';
    this.core = coreNext(this.core);
    if (!stopping) {
      this.currentTimeSeconds = 0;
    }
    this.notify();
  }

  public previousTrack() {
    if (this.core.queue.length === 0) return;
    if (this.currentTimeSeconds <= 3) {
      this.core = corePrev(this.core);
    }
    this.currentTimeSeconds = 0;
    this.notify();
  }

  public seekTo(seconds: number) {
    const duration = coreCurrentTrack(this.core)?.durationSeconds ?? 0;
    this.currentTimeSeconds = Math.max(0, Math.min(seconds, duration));
    this.notify();
  }

  public toggleShuffle() {
    this.core = coreToggleShuffle(this.core);
    this.notify();
  }

  public toggleRepeat() {
    this.core = coreToggleRepeat(this.core);
    this.notify();
  }
}

export const audioPlayer = new AudioPlayerEngine();
