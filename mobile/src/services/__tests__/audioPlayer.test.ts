import { describe, it, expect, beforeEach } from 'vitest';
import { audioPlayer } from '../audioPlayer';
import { Track } from '../api';

const mockTracks: Track[] = [
  {
    id: 1,
    title: 'Cosmic Voyager',
    artistName: 'Solaris',
    albumTitle: 'Midnight Sun',
    durationSeconds: 255,
    streamUrl: 'http://localhost/api/stream/1',
  },
  {
    id: 2,
    title: 'Digital Horizon',
    artistName: 'SynthWave',
    albumTitle: 'Neon Pulse',
    durationSeconds: 210,
    streamUrl: 'http://localhost/api/stream/2',
  },
];

describe('Shared Audio Player Engine (AC-3 & AC-4)', () => {
  beforeEach(() => {
    audioPlayer.playQueue([], 0);
  });

  it('should initialize queue and set current track on playQueue', () => {
    audioPlayer.playQueue(mockTracks, 0);

    const state = audioPlayer.getState();
    expect(state.currentTrack?.title).toBe('Cosmic Voyager');
    expect(state.isPlaying).toBe(true);
    expect(state.queue).toHaveLength(2);
    expect(state.queueIndex).toBe(0);
  });

  it('should toggle play and pause state', () => {
    audioPlayer.playQueue(mockTracks, 0);
    expect(audioPlayer.getState().isPlaying).toBe(true);

    audioPlayer.togglePlayPause();
    expect(audioPlayer.getState().isPlaying).toBe(false);

    audioPlayer.togglePlayPause();
    expect(audioPlayer.getState().isPlaying).toBe(true);
  });

  it('should advance to next track', () => {
    audioPlayer.playQueue(mockTracks, 0);
    audioPlayer.nextTrack();

    const state = audioPlayer.getState();
    expect(state.queueIndex).toBe(1);
    expect(state.currentTrack?.title).toBe('Digital Horizon');
  });

  it('should toggle repeat mode cycle (off -> all -> one)', () => {
    expect(audioPlayer.getState().repeatMode).toBe('off');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('all');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('one');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('off');
  });
});
