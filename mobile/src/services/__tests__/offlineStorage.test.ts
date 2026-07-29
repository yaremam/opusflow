import { describe, it, expect, beforeEach } from 'vitest';
import { offlineStorage } from '../offlineStorage';
import { Track } from '../api';

const mockTrack: Track = {
  id: 201,
  title: 'Cosmic Voyager',
  artistName: 'Solaris',
  albumTitle: 'Midnight Sun',
  durationSeconds: 255,
  streamUrl: 'http://localhost/api/stream/201',
};

describe('Offline Storage & Smart LRU Cache (AC-5, AC-6, AC-7)', () => {
  beforeEach(async () => {
    await offlineStorage.clearStreamCache();
    for (const item of offlineStorage.getDownloadedItems()) {
      await offlineStorage.removeTrack(item.id);
    }
  });

  it('should mark explicitly downloaded track as offline', async () => {
    await offlineStorage.downloadTrackForOffline(mockTrack);
    expect(offlineStorage.isTrackOffline(201)).toBe(true);

    const items = offlineStorage.getDownloadedItems();
    expect(items).toHaveLength(1);
    expect(items[0].isExplicit).toBe(true);
  });

  it('should calculate storage metrics correctly', async () => {
    await offlineStorage.downloadTrackForOffline(mockTrack);
    const metrics = offlineStorage.getStorageMetrics();

    expect(metrics.totalUsedBytes).toBeGreaterThan(0);
    expect(metrics.explicitDownloadBytes).toBe(metrics.totalUsedBytes);
    expect(metrics.lruCacheBytes).toBe(0);
  });

  it('should clear only stream cache entries when requested', async () => {
    await offlineStorage.downloadTrackForOffline(mockTrack);
    await offlineStorage.cacheStreamedTrack({ ...mockTrack, id: 202, title: 'Streamed Track' });

    expect(offlineStorage.getDownloadedItems()).toHaveLength(2);

    await offlineStorage.clearStreamCache();

    const items = offlineStorage.getDownloadedItems();
    expect(items).toHaveLength(1);
    expect(items[0].id).toBe(201);
  });
});
