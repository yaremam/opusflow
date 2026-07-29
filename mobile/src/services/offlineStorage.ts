import { Directory, File, Paths } from 'expo-file-system';
import { Track } from './api';
import { getServerCredentials } from './connection';

export interface StorageMetrics {
  totalUsedBytes: number;
  explicitDownloadBytes: number;
  lruCacheBytes: number;
  // Real device free space (TDR 023) — replaces the earlier fixed "2 GB"
  // quota concept entirely; there's no configured limit to report.
  availableDiskSpaceBytes: number;
}

export interface DownloadedItem {
  id: number;
  title: string;
  artistName: string;
  albumTitle: string;
  sizeBytes: number;
  downloadedAt: number;
  isExplicit: boolean;
  localAudioPath: string;
  coverUrl?: string;
  localCoverUrl?: string;
}

const CACHE_DIR_NAME = 'opusflow_audio_cache';
const MANIFEST_FILE_NAME = 'manifest.json';

// FREE_SPACE_SAFETY_MARGIN_BYTES replaces the old fixed cache-size quota
// (TDR 023): rather than capping our own cache at an arbitrary number,
// the LRU cache evicts its own oldest entries whenever real device free
// space drops below this reserve. Explicit downloads are never eviction
// candidates — only entries added by cacheStreamedTrack are.
const FREE_SPACE_SAFETY_MARGIN_BYTES = 1 * 1024 * 1024 * 1024; // 1 GB

// OfflineStorageManager persists its index to a JSON manifest file under
// the app's document directory (TDR 023 AC-3) — the same in-memory
// filter/sort logic this class always had, just no longer lost on
// restart. Exported (not just the singleton below) so tests can construct
// fresh instances against a fresh mock filesystem to verify persistence
// actually survives a "restart".
export class OfflineStorageManager {
  private items: Map<number, DownloadedItem> = new Map();
  private loaded = false;
  private readonly cacheDir: Directory;
  private readonly manifestFile: File;

  constructor() {
    this.cacheDir = new Directory(Paths.document, CACHE_DIR_NAME);
    this.manifestFile = new File(this.cacheDir, MANIFEST_FILE_NAME);
  }

  private ensureLoaded(): void {
    if (this.loaded) return;
    this.loaded = true;
    this.cacheDir.create({ intermediates: true, idempotent: true });
    if (this.manifestFile.exists) {
      try {
        const parsed = JSON.parse(this.manifestFile.textSync()) as DownloadedItem[];
        this.items = new Map(parsed.map((item) => [item.id, item]));
      } catch {
        this.items = new Map();
      }
    }
  }

  private persist(): void {
    this.cacheDir.create({ intermediates: true, idempotent: true });
    this.manifestFile.write(JSON.stringify(Array.from(this.items.values())));
  }

  private localAudioFile(track: Track): File {
    return new File(this.cacheDir, `${track.id}.audio`);
  }

  private localCoverFile(track: Track): File {
    return new File(this.cacheDir, `${track.id}.cover`);
  }

  // File.downloadFileAsync makes its own native HTTP request, entirely
  // outside api.ts's fetch() layer, so it never picks up a token any other
  // way — same reason audioPlayer.ts has to attach this explicitly too.
  private async downloadAudioAndCover(
    track: Track
  ): Promise<{ sizeBytes: number; localAudioPath: string; localCoverPath: string | undefined }> {
    const creds = await getServerCredentials();
    const options = creds ? { headers: { Authorization: `Bearer ${creds.pairingToken}` } } : undefined;

    const audioFile = await File.downloadFileAsync(track.streamUrl, this.localAudioFile(track), options);
    let localCoverPath: string | undefined;
    if (track.coverUrl) {
      const coverFile = await File.downloadFileAsync(track.coverUrl, this.localCoverFile(track), options);
      localCoverPath = coverFile.uri;
    }
    return { sizeBytes: audioFile.size, localAudioPath: audioFile.uri, localCoverPath };
  }

  /**
   * Downloads a track's real audio file (and its artwork, if any) for
   * offline playback (AC-2) — a simple one-shot download; a failure just
   * requires tapping "download" again, nothing resumable is persisted.
   */
  public async downloadTrackForOffline(track: Track): Promise<DownloadedItem> {
    this.ensureLoaded();
    const { sizeBytes, localAudioPath, localCoverPath } = await this.downloadAudioAndCover(track);

    const item: DownloadedItem = {
      id: track.id,
      title: track.title,
      artistName: track.artistName,
      albumTitle: track.albumTitle,
      sizeBytes,
      downloadedAt: Date.now(),
      isExplicit: true,
      localAudioPath,
      coverUrl: track.coverUrl,
      localCoverUrl: localCoverPath,
    };

    track.localCoverUrl = localCoverPath;
    this.items.set(track.id, item);
    this.persist();
    return item;
  }

  /**
   * Caches a streamed (not explicitly downloaded) track once it finishes
   * playing (AC-5) — invisible/automatic, on any network, distinct from
   * an explicit download only in that it's an eviction candidate.
   */
  public async cacheStreamedTrack(track: Track): Promise<void> {
    this.ensureLoaded();
    if (this.items.has(track.id)) return;

    const { sizeBytes, localAudioPath, localCoverPath } = await this.downloadAudioAndCover(track);

    const item: DownloadedItem = {
      id: track.id,
      title: track.title,
      artistName: track.artistName,
      albumTitle: track.albumTitle,
      sizeBytes,
      downloadedAt: Date.now(),
      isExplicit: false,
      localAudioPath,
      coverUrl: track.coverUrl,
      localCoverUrl: localCoverPath,
    };

    track.localCoverUrl = localCoverPath;
    this.items.set(track.id, item);
    this.persist();
    this.evictLruUntilSafeMargin();
  }

  public getDownloadedItems(): DownloadedItem[] {
    this.ensureLoaded();
    return Array.from(this.items.values());
  }

  public isTrackOffline(trackId: number): boolean {
    this.ensureLoaded();
    return this.items.has(trackId);
  }

  public getLocalCoverUrl(trackId: number): string | undefined {
    this.ensureLoaded();
    return this.items.get(trackId)?.localCoverUrl;
  }

  /**
   * The real local audio file path for a track, if one exists — what
   * audioPlayer checks to play from disk instead of streaming (AC-4).
   */
  public getLocalAudioPath(trackId: number): string | undefined {
    this.ensureLoaded();
    return this.items.get(trackId)?.localAudioPath;
  }

  private deleteItemFiles(item: DownloadedItem): void {
    const audioFile = new File(item.localAudioPath);
    if (audioFile.exists) audioFile.delete();
    if (item.localCoverUrl) {
      const coverFile = new File(item.localCoverUrl);
      if (coverFile.exists) coverFile.delete();
    }
  }

  public async removeTrack(trackId: number): Promise<void> {
    this.ensureLoaded();
    const item = this.items.get(trackId);
    if (!item) return;
    this.deleteItemFiles(item);
    this.items.delete(trackId);
    this.persist();
  }

  public async clearStreamCache(): Promise<void> {
    this.ensureLoaded();
    for (const [id, item] of this.items.entries()) {
      if (!item.isExplicit) {
        this.deleteItemFiles(item);
        this.items.delete(id);
      }
    }
    this.persist();
  }

  public getStorageMetrics(): StorageMetrics {
    this.ensureLoaded();
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
      availableDiskSpaceBytes: Paths.availableDiskSpace,
    };
  }

  // Free-space-aware eviction (TDR 023 AC-6): no fixed quota — evict this
  // cache's own oldest LRU entries (real files, not just manifest rows)
  // only while real device free space is below the safety margin.
  // Explicit downloads are never candidates; if there's nothing left to
  // evict, this simply stops, even if still below the margin.
  private evictLruUntilSafeMargin(): void {
    if (Paths.availableDiskSpace >= FREE_SPACE_SAFETY_MARGIN_BYTES) return;

    const lruItems = Array.from(this.items.values())
      .filter((i) => !i.isExplicit)
      .sort((a, b) => a.downloadedAt - b.downloadedAt);

    for (const item of lruItems) {
      if (Paths.availableDiskSpace >= FREE_SPACE_SAFETY_MARGIN_BYTES) break;
      this.deleteItemFiles(item);
      this.items.delete(item.id);
    }
    this.persist();
  }
}

export const offlineStorage = new OfflineStorageManager();
