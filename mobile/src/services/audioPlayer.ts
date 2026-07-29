import { createAudioPlayer, setAudioModeAsync, type AudioPlayer as ExpoAudioPlayer, type AudioStatus } from 'expo-audio';
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
import { offlineStorage } from './offlineStorage';

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
// shared queue state machine (backlog/019 AC-4) — everything about *which*
// track is playing next (advance, repeat, shuffle) is the shared reducer;
// this class owns the genuinely platform-specific parts: the real
// expo-audio player, current playback time/duration (kept out of the
// shared core the same way web's TimeContext keeps it out of
// PlayerContext), lock-screen metadata, local-first source selection
// (TDR 023 AC-4), and caching a streamed track once it finishes playing
// (TDR 023 AC-5).
class AudioPlayerEngine {
  private core: QueueState<Track> = initialQueueState<Track>();
  private currentTimeSeconds = 0;
  private durationSeconds = 0;
  private isPlaying = false;
  private volume = 1.0;

  private listeners: Set<StateChangeListener> = new Set();
  private player: ExpoAudioPlayer;
  private cachedForCurrentTrack = false;

  constructor() {
    setAudioModeAsync({
      playsInSilentMode: true,
      shouldPlayInBackground: true,
      // Required for setActiveForLockScreen to associate lock-screen
      // controls with this player at all — see expo-audio's own docs.
      interruptionMode: 'doNotMix',
    }).catch(() => {
      // Best-effort: a misconfigured/unsupported platform shouldn't crash
      // playback, it just won't get background/lock-screen behavior.
    });

    this.player = createAudioPlayer(null);
    this.player.addListener('playbackStatusUpdate', (status: AudioStatus) => {
      this.onStatusUpdate(status);
    });
  }

  public getState(): AudioPlayerState {
    const track = coreCurrentTrack(this.core);
    return {
      currentTrack: track,
      queue: this.core.queue,
      queueIndex: this.core.currentIndex,
      isPlaying: this.isPlaying,
      currentTimeSeconds: this.currentTimeSeconds,
      durationSeconds: this.durationSeconds || track?.durationSeconds || 0,
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

  // sourceFor picks the local file over the network stream whenever one
  // exists (TDR 023 AC-4) — no connectivity check needed; a network
  // fallback that later fails just surfaces as a normal playback error.
  private sourceFor(track: Track): { uri: string } {
    const localPath = offlineStorage.getLocalAudioPath(track.id);
    return { uri: localPath ?? track.streamUrl };
  }

  private loadCurrentTrack() {
    const track = coreCurrentTrack(this.core);
    this.currentTimeSeconds = 0;
    this.durationSeconds = track?.durationSeconds ?? 0;
    this.cachedForCurrentTrack = false;

    if (!track) {
      this.player.pause();
      this.player.clearLockScreenControls();
      return;
    }

    this.player.replace(this.sourceFor(track));
    this.player.play();
    this.player.setActiveForLockScreen(true, {
      title: track.title,
      artist: track.artistName,
      albumTitle: track.albumTitle,
      artworkUrl: track.localCoverUrl || track.coverUrl,
    });
  }

  private onStatusUpdate(status: AudioStatus) {
    this.isPlaying = status.playing;
    this.currentTimeSeconds = status.currentTime;
    if (status.duration > 0) {
      this.durationSeconds = status.duration;
    }

    if (status.didJustFinish) {
      this.cacheCurrentTrackIfNeeded();
      this.nextTrack();
      return;
    }

    this.notify();
  }

  // Caches a streamed (not already-offline) track once it finishes
  // playing (TDR 023 AC-5) — invisible/automatic, on any network.
  // cacheStreamedTrack itself already no-ops for a track that's already
  // downloaded/cached, so this is safe to call unconditionally.
  private cacheCurrentTrackIfNeeded() {
    const track = coreCurrentTrack(this.core);
    if (!track || this.cachedForCurrentTrack) return;
    this.cachedForCurrentTrack = true;
    offlineStorage.cacheStreamedTrack(track).catch(() => {
      // Best-effort background caching — a failure here shouldn't disrupt
      // playback, which has already moved on by the time this settles.
    });
  }

  public playQueue(tracks: Track[], startIndex: number = 0) {
    const next = corePlayFrom(this.core, tracks, startIndex);
    if (next === this.core) return;
    this.core = next;
    this.loadCurrentTrack();
    this.notify();
  }

  public togglePlayPause() {
    if (!coreCurrentTrack(this.core) && this.core.queue.length > 0) {
      this.playQueue(this.core.queue, 0);
      return;
    }
    if (this.player.playing) {
      this.player.pause();
    } else {
      this.player.play();
    }
  }

  public nextTrack() {
    if (this.core.queue.length === 0) return;
    const stopping = !this.core.isShuffle && this.core.currentIndex >= this.core.queue.length - 1 && this.core.repeatMode !== 'all';
    this.core = coreNext(this.core);
    if (stopping) {
      this.notify();
      return;
    }
    this.loadCurrentTrack();
    this.notify();
  }

  public previousTrack() {
    if (this.core.queue.length === 0) return;
    if (this.currentTimeSeconds > 3) {
      this.player.seekTo(0);
      return;
    }
    this.core = corePrev(this.core);
    this.loadCurrentTrack();
    this.notify();
  }

  public seekTo(seconds: number) {
    const duration = this.durationSeconds || coreCurrentTrack(this.core)?.durationSeconds || 0;
    this.player.seekTo(Math.max(0, Math.min(seconds, duration)));
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
