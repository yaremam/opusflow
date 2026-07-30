import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { ActivityIndicator, FlatList, Image, StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { Album, fetchAlbumTracks, Track } from '../services/api';
import { audioPlayer } from '../services/audioPlayer';
import { offlineStorage } from '../services/offlineStorage';
import { ACCENT } from '../theme';

interface AlbumDetailScreenProps {
  album: Album;
  onBack: () => void;
  onOpenPlayer?: () => void;
}

// AlbumDetailScreen matches web's AlbumDetailPage (backlog/026 AC-4):
// per-track play/add-to-queue/download, plus a new "Download album"
// action (AC-5) that downloads every track via the existing per-track
// download path, showing a spinner and live "X of Y" progress.
export function AlbumDetailScreen({ album, onBack, onOpenPlayer }: AlbumDetailScreenProps) {
  const [tracks, setTracks] = useState<Track[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [offlineMap, setOfflineMap] = useState<Record<number, boolean>>({});
  const [justQueued, setJustQueued] = useState<Record<number, boolean>>({});
  const [downloadingAlbum, setDownloadingAlbum] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState({ done: 0, total: 0 });

  const refreshOfflineMap = (list: Track[]) => {
    const map: Record<number, boolean> = {};
    for (const t of list) map[t.id] = offlineStorage.isTrackOffline(t.id);
    setOfflineMap(map);
  };

  useEffect(() => {
    let cancelled = false;
    fetchAlbumTracks(album)
      .then((result) => {
        if (cancelled) return;
        setTracks(result);
        refreshOfflineMap(result);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Could not load this album.');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [album.id]);

  const handlePlayTrack = (index: number) => {
    audioPlayer.playQueue(tracks, index);
    onOpenPlayer?.();
  };

  const handleAddToQueue = (track: Track) => {
    audioPlayer.addToQueue(track);
    setJustQueued((prev) => ({ ...prev, [track.id]: true }));
    setTimeout(() => setJustQueued((prev) => ({ ...prev, [track.id]: false })), 1500);
  };

  const handleToggleOffline = async (track: Track) => {
    if (offlineMap[track.id]) {
      await offlineStorage.removeTrack(track.id);
    } else {
      await offlineStorage.downloadTrackForOffline(track);
    }
    refreshOfflineMap(tracks);
  };

  const handleDownloadAlbum = async () => {
    const toDownload = tracks.filter((t) => !offlineStorage.isTrackOffline(t.id));
    if (toDownload.length === 0) return;

    setDownloadingAlbum(true);
    setDownloadProgress({ done: 0, total: toDownload.length });
    for (const track of toDownload) {
      try {
        await offlineStorage.downloadTrackForOffline(track);
      } catch {
        // Best-effort: one failed track shouldn't stop the rest of the
        // album — its row just stays showing the download icon, so it's
        // obvious it didn't complete and can be retried individually.
      }
      setDownloadProgress((prev) => ({ ...prev, done: prev.done + 1 }));
      refreshOfflineMap(tracks);
    }
    setDownloadingAlbum(false);
  };

  const allDownloaded = tracks.length > 0 && tracks.every((t) => offlineMap[t.id]);

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onBack}>
          <Ionicons name="chevron-back" size={18} color="#f3f4f6" />
        </TouchableOpacity>
        <Text style={styles.topTitle}>ALBUM</Text>
        <View style={styles.iconBtn} />
      </View>

      {error ? (
        <View style={styles.emptyState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : (
        <FlatList
          data={tracks}
          keyExtractor={(item) => item.id.toString()}
          contentContainerStyle={styles.listContent}
          ListHeaderComponent={
            <>
              <View style={styles.hero}>
                {album.coverUrl ? (
                  <Image source={{ uri: album.coverUrl }} style={styles.heroCover} />
                ) : (
                  <View style={[styles.heroCover, styles.heroCoverPlaceholder]}>
                    <Ionicons name="disc-outline" size={36} color="#ffffff" />
                  </View>
                )}
                <View style={styles.heroText}>
                  <Text style={styles.heroTitle} numberOfLines={2}>{album.title}</Text>
                  <Text style={styles.heroSub}>
                    {album.artistName}
                    {album.year ? ` · ${album.year}` : ''}
                    {tracks.length > 0 ? ` · ${tracks.length} songs` : ''}
                  </Text>
                </View>
              </View>

              {!loading && tracks.length > 0 && (
                <TouchableOpacity
                  style={[styles.downloadAlbumBtn, (downloadingAlbum || allDownloaded) && styles.downloadAlbumBtnMuted]}
                  onPress={handleDownloadAlbum}
                  disabled={downloadingAlbum || allDownloaded}
                >
                  {downloadingAlbum ? (
                    <>
                      <ActivityIndicator size="small" color="#9ca3af" />
                      <Text style={styles.downloadAlbumTextMuted}>
                        {downloadProgress.done} of {downloadProgress.total} downloaded…
                      </Text>
                    </>
                  ) : allDownloaded ? (
                    <>
                      <Ionicons name="checkmark-circle" size={16} color="#9ca3af" />
                      <Text style={styles.downloadAlbumTextMuted}>Album downloaded</Text>
                    </>
                  ) : (
                    <>
                      <Ionicons name="download-outline" size={16} color={ACCENT} />
                      <Text style={styles.downloadAlbumText}>Download album</Text>
                    </>
                  )}
                </TouchableOpacity>
              )}
            </>
          }
          ListEmptyComponent={loading ? <Text style={styles.emptyText}>Loading…</Text> : <Text style={styles.emptyText}>No tracks.</Text>}
          renderItem={({ item, index }) => (
            <TouchableOpacity style={styles.trackRow} onPress={() => handlePlayTrack(index)}>
              <Text style={styles.trackNum}>{index + 1}</Text>
              <View style={styles.trackText}>
                <Text style={styles.trackTitle} numberOfLines={1}>{item.title}</Text>
              </View>
              <Text style={styles.trackDur}>
                {Math.floor(item.durationSeconds / 60)}:{String(item.durationSeconds % 60).padStart(2, '0')}
              </Text>
              <View style={styles.trackActions}>
                <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleAddToQueue(item)}>
                  {justQueued[item.id] ? (
                    <Ionicons name="checkmark-circle" size={16} color="#10b981" />
                  ) : (
                    <Ionicons name="add-circle-outline" size={16} color="#9ca3af" />
                  )}
                </TouchableOpacity>
                <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleToggleOffline(item)}>
                  {offlineMap[item.id] ? (
                    <Ionicons name="checkmark-circle" size={16} color="#10b981" />
                  ) : (
                    <Ionicons name="download-outline" size={16} color="#9ca3af" />
                  )}
                </TouchableOpacity>
              </View>
            </TouchableOpacity>
          )}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0f131d', paddingTop: 40 },
  topBar: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingHorizontal: 16,
    marginBottom: 8,
  },
  iconBtn: {
    width: 36,
    height: 36,
    borderRadius: 12,
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  topTitle: { fontSize: 12, fontWeight: '700', color: '#9ca3af', letterSpacing: 0.5 },
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  emptyText: { color: '#6b7280', fontSize: 13, textAlign: 'center', marginTop: 20 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  listContent: { paddingHorizontal: 16, paddingBottom: 100 },
  hero: { flexDirection: 'row', gap: 12, marginBottom: 14, alignItems: 'flex-end' },
  heroCover: { width: 88, height: 88, borderRadius: 12 },
  heroCoverPlaceholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
  heroText: { flex: 1 },
  heroTitle: { fontSize: 16, fontWeight: '700', color: '#f3f4f6' },
  heroSub: { fontSize: 11, color: '#9ca3af', marginTop: 3 },
  downloadAlbumBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 6,
    backgroundColor: 'rgba(45, 217, 196, 0.12)',
    borderWidth: 1,
    borderColor: ACCENT,
    borderRadius: 12,
    paddingVertical: 10,
    marginBottom: 14,
  },
  downloadAlbumBtnMuted: { backgroundColor: '#141824', borderColor: 'rgba(255, 255, 255, 0.08)' },
  downloadAlbumText: { fontSize: 13, fontWeight: '700', color: ACCENT },
  downloadAlbumTextMuted: { fontSize: 13, fontWeight: '600', color: '#9ca3af' },
  trackRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: 'rgba(255, 255, 255, 0.08)',
  },
  trackNum: { width: 18, fontSize: 11, color: '#6b7280', textAlign: 'center' },
  trackText: { flex: 1 },
  trackTitle: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  trackDur: { fontSize: 11, color: '#9ca3af' },
  trackActions: { flexDirection: 'row', alignItems: 'center', gap: 2 },
  trackActionBtn: { padding: 5 },
});
