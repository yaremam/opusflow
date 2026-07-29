import { Track } from './api';

export type RepeatMode = 'off' | 'one' | 'all';

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

class AudioPlayerEngine {
  private state: AudioPlayerState = {
    currentTrack: null,
    queue: [],
    queueIndex: -1,
    isPlaying: false,
    currentTimeSeconds: 0,
    durationSeconds: 0,
    volume: 1.0,
    repeatMode: 'off',
    isShuffle: false,
  };

  private listeners: Set<StateChangeListener> = new Set();

  public getState(): AudioPlayerState {
    return { ...this.state };
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
    if (tracks.length === 0) return;
    this.state.queue = [...tracks];
    this.state.queueIndex = Math.max(0, Math.min(startIndex, tracks.length - 1));
    this.state.currentTrack = this.state.queue[this.state.queueIndex];
    this.state.isPlaying = true;
    this.state.currentTimeSeconds = 0;
    this.state.durationSeconds = this.state.currentTrack.durationSeconds;
    this.notify();
  }

  public togglePlayPause() {
    if (!this.state.currentTrack && this.state.queue.length > 0) {
      this.playQueue(this.state.queue, 0);
      return;
    }
    this.state.isPlaying = !this.state.isPlaying;
    this.notify();
  }

  public nextTrack() {
    if (this.state.queue.length === 0) return;

    if (this.state.isShuffle) {
      this.state.queueIndex = Math.floor(Math.random() * this.state.queue.length);
    } else if (this.state.queueIndex < this.state.queue.length - 1) {
      this.state.queueIndex++;
    } else if (this.state.repeatMode === 'all') {
      this.state.queueIndex = 0;
    } else {
      this.state.isPlaying = false;
      this.notify();
      return;
    }

    this.state.currentTrack = this.state.queue[this.state.queueIndex];
    this.state.currentTimeSeconds = 0;
    this.state.durationSeconds = this.state.currentTrack.durationSeconds;
    this.state.isPlaying = true;
    this.notify();
  }

  public previousTrack() {
    if (this.state.queue.length === 0) return;

    if (this.state.currentTimeSeconds > 3) {
      this.state.currentTimeSeconds = 0;
    } else if (this.state.queueIndex > 0) {
      this.state.queueIndex--;
      this.state.currentTrack = this.state.queue[this.state.queueIndex];
      this.state.currentTimeSeconds = 0;
      this.state.durationSeconds = this.state.currentTrack.durationSeconds;
    } else {
      this.state.currentTimeSeconds = 0;
    }
    this.notify();
  }

  public seekTo(seconds: number) {
    this.state.currentTimeSeconds = Math.max(0, Math.min(seconds, this.state.durationSeconds));
    this.notify();
  }

  public toggleShuffle() {
    this.state.isShuffle = !this.state.isShuffle;
    this.notify();
  }

  public toggleRepeat() {
    const modes: RepeatMode[] = ['off', 'all', 'one'];
    const currentIndex = modes.indexOf(this.state.repeatMode);
    this.state.repeatMode = modes[(currentIndex + 1) % modes.length];
    this.notify();
  }
}

export const audioPlayer = new AudioPlayerEngine();
