import { useState, useEffect } from 'react';
import {
  StyleSheet,
  Text,
  View,
  TextInput,
  TouchableOpacity,
  ScrollView,
  FlatList,
} from 'react-native';
import { fetchCatalogAlbums, fetchCatalogTracks, Album, Track } from '../services/api';
import { audioPlayer } from '../services/audioPlayer';
import { offlineStorage } from '../services/offlineStorage';

interface LibraryScreenProps {
  onOpenPlayer?: () => void;
}

export function LibraryScreen({ onOpenPlayer }: LibraryScreenProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [albums, setAlbums] = useState<Album[]>([]);
  const [tracks, setTracks] = useState<Track[]>([]);
  const [offlineMap, setOfflineMap] = useState<Record<number, boolean>>({});

  useEffect(() => {
    loadData();
  }, [searchQuery]);

  const loadData = async () => {
    try {
      const albumData = await fetchCatalogAlbums(searchQuery);
      const trackData = await fetchCatalogTracks(searchQuery);
      setAlbums(albumData);
      setTracks(trackData);

      const map: Record<number, boolean> = {};
      for (const t of trackData) {
        map[t.id] = offlineStorage.isTrackOffline(t.id);
      }
      setOfflineMap(map);
    } catch (e) {
      const mockAlbums: Album[] = [
        { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026 },
        { id: 2, title: 'Neon Pulse', artistName: 'SynthWave', year: 2026 },
      ];
      const mockTracks: Track[] = [
        {
          id: 101,
          title: 'Cosmic Voyager',
          artistName: 'Solaris',
          albumTitle: 'Midnight Sun',
          durationSeconds: 255,
          streamUrl: 'http://localhost/api/stream/101',
        },
        {
          id: 102,
          title: 'Digital Horizon',
          artistName: 'SynthWave',
          albumTitle: 'Neon Pulse',
          durationSeconds: 210,
          streamUrl: 'http://localhost/api/stream/102',
        },
      ];
      setAlbums(mockAlbums);
      setTracks(mockTracks);
    }
  };

  const handlePlayTrack = (track: Track, index: number) => {
    audioPlayer.playQueue(tracks, index);
    onOpenPlayer?.();
  };

  const handleToggleOffline = async (track: Track) => {
    if (offlineMap[track.id]) {
      await offlineStorage.removeTrack(track.id);
    } else {
      await offlineStorage.downloadTrackForOffline(track);
    }
    setOfflineMap({ ...offlineMap, [track.id]: !offlineMap[track.id] });
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Library</Text>
        <TextInput
          style={styles.searchInput}
          placeholder="🔍 Search tracks or albums..."
          placeholderTextColor="#6b7280"
          value={searchQuery}
          onChangeText={setSearchQuery}
        />
      </View>

      <ScrollView contentContainerStyle={styles.scrollContent}>
        <Text style={styles.sectionTitle}>FEATURED ALBUMS</Text>
        <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.albumRow}>
          {albums.map((album) => (
            <TouchableOpacity key={album.id} style={styles.albumCard}>
              <View style={styles.albumCoverPlaceholder}>
                <Text style={{ fontSize: 32 }}>🌌</Text>
              </View>
              <Text style={styles.albumTitle} numberOfLines={1}>
                {album.title}
              </Text>
              <Text style={styles.albumArtist}>{album.artistName}</Text>
            </TouchableOpacity>
          ))}
        </ScrollView>

        <Text style={styles.sectionTitle}>RECENT TRACKS</Text>
        <FlatList
          data={tracks}
          keyExtractor={(item) => item.id.toString()}
          scrollEnabled={false}
          renderItem={({ item, index }) => (
            <TouchableOpacity
              style={styles.trackItem}
              onPress={() => handlePlayTrack(item, index)}
            >
              <View style={styles.trackInfo}>
                <View style={styles.trackIcon}>
                  <Text style={{ fontSize: 16 }}>🎵</Text>
                </View>
                <View style={styles.trackText}>
                  <Text style={styles.trackTitle}>{item.title}</Text>
                  <Text style={styles.trackSubtitle}>
                    {item.artistName} — {item.albumTitle}
                  </Text>
                </View>
              </View>

              <TouchableOpacity
                style={styles.offlineBtn}
                onPress={() => handleToggleOffline(item)}
              >
                <Text style={styles.offlineText}>
                  {offlineMap[item.id] ? '✅ Offline' : '⬇️'}
                </Text>
              </TouchableOpacity>
            </TouchableOpacity>
          )}
        />
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0f131d', paddingTop: 40 },
  header: { paddingHorizontal: 16, marginBottom: 12 },
  headerTitle: { fontSize: 24, fontWeight: '700', color: '#f3f4f6', marginBottom: 12 },
  searchInput: {
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 12,
    color: '#f3f4f6',
    fontSize: 14,
  },
  scrollContent: { paddingHorizontal: 16, paddingBottom: 100 },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '700',
    color: '#9ca3af',
    letterSpacing: 0.5,
    marginTop: 16,
    marginBottom: 12,
  },
  albumRow: { flexDirection: 'row', marginBottom: 16 },
  albumCard: {
    width: 140,
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 16,
    padding: 10,
    marginRight: 12,
  },
  albumCoverPlaceholder: {
    width: '100%',
    aspectRatio: 1,
    borderRadius: 12,
    backgroundColor: '#6366f1',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 8,
  },
  albumTitle: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  albumArtist: { fontSize: 11, color: '#9ca3af', marginTop: 2 },
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
  trackText: { flex: 1 },
  trackTitle: { fontSize: 14, fontWeight: '600', color: '#f3f4f6' },
  trackSubtitle: { fontSize: 12, color: '#9ca3af', marginTop: 2 },
  offlineBtn: { padding: 6 },
  offlineText: { fontSize: 12, color: '#10b981', fontWeight: '600' },
});
