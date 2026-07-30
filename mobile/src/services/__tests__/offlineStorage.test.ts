import { describe, it, expect, beforeEach, vi } from 'vitest';
import { Track } from '../api';
import { mockFs, resetMockFileSystem } from './mockExpoFileSystem';

vi.mock('expo-file-system', async () => {
  const { expoFileSystemMockFactory } = await import('./mockExpoFileSystem');
  return expoFileSystemMockFactory();
});

vi.mock('../connection', () => ({
  getServerCredentials: vi.fn().mockResolvedValue({
    serverUrl: 'http://localhost',
    pairingToken: 'opusflow_pt_test123',
  }),
}));

const { OfflineStorageManager } = await import('../offlineStorage');

const mockTrack: Track = {
  id: 201,
  title: 'Cosmic Voyager',
  artistName: 'Solaris',
  albumTitle: 'Midnight Sun',
  durationSeconds: 255,
  streamUrl: 'http://localhost/api/stream/201',
  format: 'flac',
  bitrateKbps: 940,
};

describe('Offline Storage & Smart LRU Cache (AC-2, AC-3, AC-5, AC-6, AC-7)', () => {
  beforeEach(() => {
    resetMockFileSystem();
  });

  async function freshStorage() {
    return new OfflineStorageManager();
  }

  it('downloads a real file and reports its real (measured) size, not an estimate', async () => {
    mockFs.downloadedByteLength = 4096;
    const storage = await freshStorage();

    const item = await storage.downloadTrackForOffline(mockTrack);

    expect(item.sizeBytes).toBe(4096);
    expect(item.isExplicit).toBe(true);
    expect(storage.isTrackOffline(201)).toBe(true);
    expect(storage.getLocalAudioPath(201)).toBeTruthy();
  });

  // Regression test for issue #68: File.downloadFileAsync makes its own
  // native HTTP request, entirely outside api.ts's fetch() layer, so a
  // download against a gated backend silently failed with no visible
  // error until the Authorization header was attached explicitly here too.
  it('sends the pairing token as a Bearer header on the download request (issue #68)', async () => {
    const storage = await freshStorage();

    await storage.downloadTrackForOffline(mockTrack);

    expect(mockFs.lastDownloadHeaders).toEqual({ Authorization: 'Bearer opusflow_pt_test123' });
  });

  it('persists across a restart — a fresh instance reads the same manifest', async () => {
    const storage = await freshStorage();
    await storage.downloadTrackForOffline(mockTrack);

    const restarted = await freshStorage();
    expect(restarted.isTrackOffline(201)).toBe(true);
    expect(restarted.getDownloadedItems()).toHaveLength(1);
  });

  it('cacheStreamedTrack adds a non-explicit (LRU) entry', async () => {
    const storage = await freshStorage();
    await storage.cacheStreamedTrack({ ...mockTrack, id: 202 });

    const items = storage.getDownloadedItems();
    expect(items).toHaveLength(1);
    expect(items[0].isExplicit).toBe(false);
  });

  it('storage metrics report real used bytes and real available disk space', async () => {
    mockFs.availableDiskSpace = 10 * 1024 * 1024 * 1024;
    mockFs.downloadedByteLength = 2048;
    const storage = await freshStorage();
    await storage.downloadTrackForOffline(mockTrack);

    const metrics = storage.getStorageMetrics();
    expect(metrics.explicitDownloadBytes).toBe(2048);
    expect(metrics.lruCacheBytes).toBe(0);
    expect(metrics.totalUsedBytes).toBe(2048);
    expect(metrics.availableDiskSpaceBytes).toBe(10 * 1024 * 1024 * 1024);
  });

  it('clearStreamCache deletes only LRU entries and their real files', async () => {
    const storage = await freshStorage();
    await storage.downloadTrackForOffline(mockTrack);
    await storage.cacheStreamedTrack({ ...mockTrack, id: 202, title: 'Streamed Track' });

    const lruPath = storage.getLocalAudioPath(202)!;
    expect(mockFs.store.has(lruPath)).toBe(true);

    await storage.clearStreamCache();

    const items = storage.getDownloadedItems();
    expect(items).toHaveLength(1);
    expect(items[0].id).toBe(201);
    expect(mockFs.store.has(lruPath)).toBe(false);
  });

  it('removeTrack deletes the real file, not just the manifest row', async () => {
    const storage = await freshStorage();
    await storage.downloadTrackForOffline(mockTrack);
    const path = storage.getLocalAudioPath(201)!;
    expect(mockFs.store.has(path)).toBe(true);

    await storage.removeTrack(201);

    expect(storage.isTrackOffline(201)).toBe(false);
    expect(mockFs.store.has(path)).toBe(false);
  });

  it('evicts the oldest LRU entry once available disk space drops below the safety margin', async () => {
    mockFs.availableDiskSpace = 2 * 1024 * 1024 * 1024; // above the 1GB margin
    mockFs.downloadedByteLength = 600 * 1024 * 1024; // large enough that freeing one entry crosses the margin
    const storage = await freshStorage();
    await storage.cacheStreamedTrack({ ...mockTrack, id: 301, title: 'Oldest' });
    await storage.cacheStreamedTrack({ ...mockTrack, id: 302, title: 'Newer' });

    // Simulate free space dropping below the safety margin by the time a
    // third track finishes streaming.
    mockFs.availableDiskSpace = 500 * 1024 * 1024; // 500MB, below the 1GB margin
    await storage.cacheStreamedTrack({ ...mockTrack, id: 303, title: 'Newest' });

    expect(storage.isTrackOffline(301)).toBe(false); // oldest evicted
    expect(storage.isTrackOffline(302)).toBe(true);
    expect(storage.isTrackOffline(303)).toBe(true);
  });

  it('never evicts explicit downloads to make room, only LRU entries', async () => {
    mockFs.availableDiskSpace = 2 * 1024 * 1024 * 1024;
    const storage = await freshStorage();
    await storage.downloadTrackForOffline({ ...mockTrack, id: 401, title: 'Explicit, keep me' });

    mockFs.availableDiskSpace = 500 * 1024 * 1024; // below margin, but nothing LRU to evict
    await storage.cacheStreamedTrack({ ...mockTrack, id: 402, title: 'New stream' });

    expect(storage.isTrackOffline(401)).toBe(true);
  });
});
