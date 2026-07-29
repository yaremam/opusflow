import { Track } from './api';

export interface StorageMetrics {
  totalUsedBytes: number;
  explicitDownloadBytes: number;
  lruCacheBytes: number;
  maxCacheSizeBytes: number;
}

export interface DownloadedItem {
  id: number;
  title: string;
  artistName: string;
  albumTitle: string;
  sizeBytes: number;
  downloadedAt: number;
  isExplicit: boolean;
}

class OfflineStorageManager {
  private items: Map<number, DownloadedItem> = new Map();
  private maxCacheSizeBytes: number = 2 * 1024 * 1024 * 1024; // 2 GB

  public async downloadTrackForOffline(track: Track): Promise<DownloadedItem> {
    const sizeBytes = track.durationSeconds * 128000; // Simulated ~128kbps size estimation
    const item: DownloadedItem = {
      id: track.id,
      title: track.title,
      artistName: track.artistName,
      albumTitle: track.albumTitle,
      sizeBytes,
      downloadedAt: Date.now(),
      isExplicit: true,
    };
    this.items.set(track.id, item);
    return item;
  }

  public async cacheStreamedTrack(track: Track): Promise<void> {
    if (this.items.has(track.id)) return;

    const sizeBytes = track.durationSeconds * 128000;
    const item: DownloadedItem = {
      id: track.id,
      title: track.title,
      artistName: track.artistName,
      albumTitle: track.albumTitle,
      sizeBytes,
      downloadedAt: Date.now(),
      isExplicit: false,
    };

    this.items.set(track.id, item);
    await this.enforceLRULimit();
  }

  public getDownloadedItems(): DownloadedItem[] {
    return Array.from(this.items.values());
  }

  public isTrackOffline(trackId: number): boolean {
    return this.items.has(trackId);
  }

  public async removeTrack(trackId: number): Promise<void> {
    this.items.delete(trackId);
  }

  public async clearStreamCache(): Promise<void> {
    for (const [id, item] of this.items.entries()) {
      if (!item.isExplicit) {
        this.items.delete(id);
      }
    }
  }

  public getStorageMetrics(): StorageMetrics {
    let explicitDownloadBytes = 0;
    let lruCacheBytes = 0;

    for (const item of this.items.values()) {
      if (item.isExplicit) {
        explicitDownloadBytes += item.sizeBytes;
      } else {
        lruCacheBytes += item.sizeBytes;
      }
    }

    return {
      totalUsedBytes: explicitDownloadBytes + lruCacheBytes,
      explicitDownloadBytes,
      lruCacheBytes,
      maxCacheSizeBytes: this.maxCacheSizeBytes,
    };
  }

  private async enforceLRULimit() {
    let metrics = this.getStorageMetrics();
    if (metrics.lruCacheBytes <= this.maxCacheSizeBytes) return;

    // Sort LRU cache entries by downloadedAt ascending (oldest first)
    const lruItems = Array.from(this.items.values())
      .filter((i) => !i.isExplicit)
      .sort((a, b) => a.downloadedAt - b.downloadedAt);

    for (const item of lruItems) {
      if (metrics.lruCacheBytes <= this.maxCacheSizeBytes) break;
      this.items.delete(item.id);
      metrics = this.getStorageMetrics();
    }
  }
}

export const offlineStorage = new OfflineStorageManager();
