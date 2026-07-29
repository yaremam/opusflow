import { describe, it, expect, beforeEach, vi } from 'vitest';
import { Track } from '../api';
import { mockAudio, resetMockAudio } from './mockExpoAudio';

vi.mock('expo-audio', async () => {
  const { expoAudioMockFactory } = await import('./mockExpoAudio');
  return expoAudioMockFactory();
});

const offlineStorageMocks = vi.hoisted(() => ({
  getLocalAudioPath: vi.fn<(id: number) => string | undefined>(),
  cacheStreamedTrack: vi.fn<(track: Track) => Promise<void>>(),
}));

vi.mock('../offlineStorage', () => ({
  offlineStorage: offlineStorageMocks,
}));

const { audioPlayer } = await import('../audioPlayer');

// audioPlayer is a module-level singleton that creates exactly one native
// player in its constructor and reuses it for every track via .replace()
// (the same pattern expo-audio's own docs recommend, rather than tearing
// down and recreating a native player per track change). That means the
// single MockAudioPlayer instance is created once at import time, above —
// resetMockAudio() below clears mockAudio.instances for hygiene, but we
// grab our reference to the one real instance before that happens.
const player = mockAudio.instances[0];

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

describe('Real Audio Player Engine (TDR 023 AC-1, AC-4, AC-5)', () => {
  beforeEach(() => {
    resetMockAudio();
    offlineStorageMocks.getLocalAudioPath.mockReset().mockReturnValue(undefined);
    offlineStorageMocks.cacheStreamedTrack.mockReset().mockResolvedValue(undefined);
    audioPlayer.playQueue([], 0);
  });

  it('actually plays audio via expo-audio, streaming from the network when nothing is downloaded', () => {
    audioPlayer.playQueue(mockTracks, 0);

    const state = audioPlayer.getState();
    expect(state.currentTrack?.title).toBe('Cosmic Voyager');
    expect(state.isPlaying).toBe(true);

    expect(player.source).toEqual({ uri: 'http://localhost/api/stream/1' });
    expect(player.playing).toBe(true);
  });

  it('plays from the local file instead of the network when one is downloaded (AC-4)', () => {
    offlineStorageMocks.getLocalAudioPath.mockImplementation((id) =>
      id === 1 ? 'file:///mock/document/opusflow_audio_cache/1.audio' : undefined
    );

    audioPlayer.playQueue(mockTracks, 0);

    expect(player.source).toEqual({ uri: 'file:///mock/document/opusflow_audio_cache/1.audio' });
  });

  it('sets lock-screen metadata for the now-playing track', () => {
    audioPlayer.playQueue(mockTracks, 0);

    expect(player.lockScreenActive).toBe(true);
    expect(player.lockScreenMetadata).toMatchObject({
      title: 'Cosmic Voyager',
      artist: 'Solaris',
      albumTitle: 'Midnight Sun',
    });
  });

  it('toggles play and pause on the real player', () => {
    audioPlayer.playQueue(mockTracks, 0);
    expect(player.playing).toBe(true);

    audioPlayer.togglePlayPause();
    expect(player.playing).toBe(false);
    expect(audioPlayer.getState().isPlaying).toBe(false);

    audioPlayer.togglePlayPause();
    expect(player.playing).toBe(true);
    expect(audioPlayer.getState().isPlaying).toBe(true);
  });

  it('advances to the next track and loads its source', () => {
    audioPlayer.playQueue(mockTracks, 0);
    audioPlayer.nextTrack();

    const state = audioPlayer.getState();
    expect(state.queueIndex).toBe(1);
    expect(state.currentTrack?.title).toBe('Digital Horizon');

    expect(player.source).toEqual({ uri: 'http://localhost/api/stream/2' });
  });

  it('toggles repeat mode cycle (off -> all -> one)', () => {
    expect(audioPlayer.getState().repeatMode).toBe('off');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('all');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('one');

    audioPlayer.toggleRepeat();
    expect(audioPlayer.getState().repeatMode).toBe('off');
  });

  it('caches a streamed track once it finishes playing, not when it starts (AC-5)', () => {
    audioPlayer.playQueue(mockTracks, 0);
    expect(offlineStorageMocks.cacheStreamedTrack).not.toHaveBeenCalled();

    player.simulateFinish();

    expect(offlineStorageMocks.cacheStreamedTrack).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, title: 'Cosmic Voyager' })
    );
  });

  it('auto-advances to the next track once the current one finishes', () => {
    audioPlayer.playQueue(mockTracks, 0);

    player.simulateFinish();

    expect(audioPlayer.getState().queueIndex).toBe(1);
    expect(audioPlayer.getState().currentTrack?.title).toBe('Digital Horizon');
  });
});
