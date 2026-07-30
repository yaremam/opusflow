import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { FlatList, Image, StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { fetchSongsPage, ListParams, Track } from '../services/api';
import { audioPlayer } from '../services/audioPlayer';
import { offlineStorage } from '../services/offlineStorage';
import { useInfiniteList } from '../hooks/useInfiniteList';
import { LibraryFilterBar } from '../components/LibraryFilterBar';

interface SongsListScreenProps {
  onOpenPlayer?: () => void;
}

// SongsListScreen is the Library hub's Songs tab (backlog/026 AC-1, AC-2)
// — real infinite-scroll + search/sort/genre/year, replacing the old
// screen's single unpaginated dump of the entire library (issue #77).
// Per-track play/add-to-queue/download actions are the same ones the old
// screen had, just moved here.
export function SongsListScreen({ onOpenPlayer }: SongsListScreenProps) {
  const [filters, setFilters] = useState<ListParams>({ sort: 'recent' });
  const { items, loading, loadingMore, error, hasMore, loadMore } = useInfiniteList(fetchSongsPage, filters);
  const [offlineMap, setOfflineMap] = useState<Record<number, boolean>>({});
  const [justQueued, setJustQueued] = useState<Record<number, boolean>>({});

  useEffect(() => {
    const map: Record<number, boolean> = {};
    for (const t of items) map[t.id] = offlineStorage.isTrackOffline(t.id);
    setOfflineMap(map);
  }, [items]);

  const handlePlayTrack = (track: Track, index: number) => {
    audioPlayer.playQueue(items, index);
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
    setOfflineMap((prev) => ({ ...prev, [track.id]: !prev[track.id] }));
  };

  return (
    <View style={styles.container}>
      <LibraryFilterBar filters={filters} onChange={setFilters} searchPlaceholder="Search songs…" />

      {error ? (
        <View style={styles.errorState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : loading ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyText}>Loading…</Text>
        </View>
      ) : items.length === 0 ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyText}>No songs yet.</Text>
        </View>
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.id.toString()}
          contentContainerStyle={styles.listContent}
          onEndReached={loadMore}
          onEndReachedThreshold={0.4}
          ListFooterComponent={loadingMore ? <Text style={styles.footerText}>Loading more…</Text> : null}
          renderItem={({ item, index }) => (
            <TouchableOpacity style={styles.trackItem} onPress={() => handlePlayTrack(item, index)}>
              <View style={styles.trackInfo}>
                {item.coverUrl ? (
                  <Image source={{ uri: item.coverUrl }} style={styles.trackArt} />
                ) : (
                  <View style={styles.trackIcon}>
                    <Ionicons name="musical-notes-outline" size={16} color="#9ca3af" />
                  </View>
                )}
                <View style={styles.trackText}>
                  <Text style={styles.trackTitle}>{item.title}</Text>
                  <Text style={styles.trackSubtitle}>
                    {item.artistName} — {item.albumTitle}
                  </Text>
                </View>
              </View>

              <View style={styles.trackActions}>
                <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleAddToQueue(item)}>
                  {justQueued[item.id] ? (
                    <Ionicons name="checkmark-circle" size={18} color="#10b981" />
                  ) : (
                    <Ionicons name="add-circle-outline" size={18} color="#9ca3af" />
                  )}
                </TouchableOpacity>
                <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleToggleOffline(item)}>
                  {offlineMap[item.id] ? (
                    <Ionicons name="checkmark-circle" size={18} color="#10b981" />
                  ) : (
                    <Ionicons name="download-outline" size={18} color="#9ca3af" />
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
  container: { flex: 1, paddingHorizontal: 16 },
  listContent: { paddingTop: 8, paddingBottom: 100 },
  errorState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  emptyText: { color: '#6b7280', fontSize: 13 },
  footerText: { color: '#6b7280', fontSize: 12, textAlign: 'center', paddingVertical: 16 },
  trackItem: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 12,
    marginBottom: 8,
  },
  trackInfo: { flexDirection: 'row', alignItems: 'center', flex: 1 },
  trackIcon: {
    width: 36,
    height: 36,
    borderRadius: 8,
    backgroundColor: 'rgba(255, 255, 255, 0.05)',
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 12,
  },
  trackArt: { width: 36, height: 36, borderRadius: 8, marginRight: 12 },
  trackText: { flex: 1 },
  trackTitle: { fontSize: 14, fontWeight: '600', color: '#f3f4f6' },
  trackSubtitle: { fontSize: 12, color: '#9ca3af', marginTop: 2 },
  trackActions: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  trackActionBtn: { padding: 6 },
});
