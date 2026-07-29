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

// Regression coverage for issue #70: expo-audio's native player makes its
// own HTTP request, entirely outside api.ts's fetch() layer, so streaming
// a track against a gated backend silently failed with no visible error
// until the Authorization header was attached explicitly in sourceFor().
vi.mock('../connection', () => ({
  getServerCredentials: vi.fn().mockResolvedValue({
    serverUrl: 'http://localhost',
    pairingToken: 'opusflow_pt_test123',
  }),
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
  beforeEach(async () => {
    resetMockAudio();
    offlineStorageMocks.getLocalAudioPath.mockReset().mockReturnValue(undefined);
    offlineStorageMocks.cacheStreamedTrack.mockReset().mockResolvedValue(undefined);
    await audioPlayer.playQueue([], 0);
  });

  it('actually plays audio via expo-audio, streaming from the network when nothing is downloaded', async () => {
    await audioPlayer.playQueue(mockTracks, 0);

    const state = audioPlayer.getState();
    expect(state.currentTrack?.title).toBe('Cosmic Voyager');
    expect(state.isPlaying).toBe(true);

    expect(player.source).toEqual({
      uri: 'http://localhost/api/stream/1',
      headers: { Authorization: 'Bearer opusflow_pt_test123' },
    });
    expect(player.playing).toBe(true);
  });

  it('plays from the local file instead of the network when one is downloaded (AC-4)', async () => {
    offlineStorageMocks.getLocalAudioPath.mockImplementation((id) =>
      id === 1 ? 'file:///mock/document/opusflow_audio_cache/1.audio' : undefined
    );

    await audioPlayer.playQueue(mockTracks, 0);

    expect(player.source).toEqual({ uri: 'file:///mock/document/opusflow_audio_cache/1.audio' });
  });

  it('sets lock-screen metadata for the now-playing track', async () => {
    await audioPlayer.playQueue(mockTracks, 0);

    expect(player.lockScreenActive).toBe(true);
    expect(player.lockScreenMetadata).toMatchObject({
      title: 'Cosmic Voyager',
      artist: 'Solaris',
      albumTitle: 'Midnight Sun',
    });
  });

  it('toggles play and pause on the real player', async () => {
    await audioPlayer.playQueue(mockTracks, 0);
    expect(player.playing).toBe(true);

    audioPlayer.togglePlayPause();
    expect(player.playing).toBe(false);
    expect(audioPlayer.getState().isPlaying).toBe(false);

    audioPlayer.togglePlayPause();
    expect(player.playing).toBe(true);
    expect(audioPlayer.getState().isPlaying).toBe(true);
  });

  it('advances to the next track and loads its source', async () => {
    await audioPlayer.playQueue(mockTracks, 0);
    await audioPlayer.nextTrack();

    const state = audioPlayer.getState();
    expect(state.queueIndex).toBe(1);
    expect(state.currentTrack?.title).toBe('Digital Horizon');

    expect(player.source).toEqual({
      uri: 'http://localhost/api/stream/2',
      headers: { Authorization: 'Bearer opusflow_pt_test123' },
    });
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

  it('caches a streamed track once it finishes playing, not when it starts (AC-5)', async () => {
    await audioPlayer.playQueue(mockTracks, 0);
    expect(offlineStorageMocks.cacheStreamedTrack).not.toHaveBeenCalled();

    player.simulateFinish();

    await vi.waitFor(() => {
      expect(offlineStorageMocks.cacheStreamedTrack).toHaveBeenCalledWith(
        expect.objectContaining({ id: 1, title: 'Cosmic Voyager' })
      );
    });
  });

  it('auto-advances to the next track once the current one finishes', async () => {
    await audioPlayer.playQueue(mockTracks, 0);

    player.simulateFinish();

    await vi.waitFor(() => {
      expect(audioPlayer.getState().queueIndex).toBe(1);
    });
    expect(audioPlayer.getState().currentTrack?.title).toBe('Digital Horizon');
  });
});
