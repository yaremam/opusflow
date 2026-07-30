import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TextInput, TouchableOpacity, ScrollView, FlatList, Image } from 'react-native';
import { fetchAlbumTracks, fetchCatalogAlbums, fetchCatalogTracks, Album, Track } from '../services/api';
import { audioPlayer } from '../services/audioPlayer';
import { offlineStorage } from '../services/offlineStorage';
import { ACCENT } from '../theme';

interface LibraryScreenProps {
  onOpenPlayer?: () => void;
}

export function LibraryScreen({ onOpenPlayer }: LibraryScreenProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [albums, setAlbums] = useState<Album[]>([]);
  const [tracks, setTracks] = useState<Track[]>([]);
  const [offlineMap, setOfflineMap] = useState<Record<number, boolean>>({});
  const [loadError, setLoadError] = useState<string | null>(null);
  // Tracks whose "add to queue" icon should briefly show a checkmark
  // (backlog/025 AC-5) — the action has no other visible effect on this
  // screen, so without this it would look like tapping did nothing.
  const [justQueued, setJustQueued] = useState<Record<number, boolean>>({});

  useEffect(() => {
    loadData();
  }, [searchQuery]);

  const loadData = async () => {
    try {
      const albumData = await fetchCatalogAlbums(searchQuery);
      const trackData = await fetchCatalogTracks(searchQuery);
      setAlbums(albumData);
      setTracks(trackData);
      setLoadError(null);

      const map: Record<number, boolean> = {};
      for (const t of trackData) {
        map[t.id] = offlineStorage.isTrackOffline(t.id);
      }
      setOfflineMap(map);
    } catch (e) {
      setAlbums([]);
      setTracks([]);
      setLoadError(e instanceof Error ? e.message : 'Could not load your library.');
    }
  };

  const handlePlayTrack = (track: Track, index: number) => {
    audioPlayer.playQueue(tracks, index);
    onOpenPlayer?.();
  };

  const handlePlayAlbum = async (album: Album) => {
    try {
      const albumTracks = await fetchAlbumTracks(album);
      if (albumTracks.length === 0) return;
      audioPlayer.playQueue(albumTracks, 0);
      onOpenPlayer?.();
    } catch (e) {
      console.error('Failed to load album tracks:', e);
    }
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
    setOfflineMap({ ...offlineMap, [track.id]: !offlineMap[track.id] });
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Library</Text>
        <View style={styles.searchRow}>
          <Ionicons name="search-outline" size={16} color="#6b7280" style={styles.searchIcon} />
          <TextInput
            style={styles.searchInput}
            placeholder="Search tracks or albums..."
            placeholderTextColor="#6b7280"
            value={searchQuery}
            onChangeText={setSearchQuery}
          />
        </View>
      </View>

      {loadError ? (
        <View style={styles.errorState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorTitle}>Couldn't reach your library</Text>
          <Text style={styles.errorText}>{loadError}</Text>
          <Text style={styles.errorHint}>Check your server connection on the Connect tab.</Text>
        </View>
      ) : (
        <ScrollView contentContainerStyle={styles.scrollContent}>
          <Text style={styles.sectionTitle}>FEATURED ALBUMS</Text>
          {albums.length === 0 ? (
            <Text style={styles.emptyText}>No albums yet.</Text>
          ) : (
            <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.albumRow}>
              {albums.map((album) => (
                <TouchableOpacity key={album.id} style={styles.albumCard} onPress={() => handlePlayAlbum(album)}>
                  {album.coverUrl ? (
                    <Image source={{ uri: album.coverUrl }} style={styles.albumCover} />
                  ) : (
                    <View style={styles.albumCoverPlaceholder}>
                      <Ionicons name="disc-outline" size={30} color="#ffffff" />
                    </View>
                  )}
                  <Text style={styles.albumTitle} numberOfLines={1}>
                    {album.title}
                  </Text>
                  <Text style={styles.albumArtist}>{album.artistName}</Text>
                </TouchableOpacity>
              ))}
            </ScrollView>
          )}

          <Text style={styles.sectionTitle}>RECENT TRACKS</Text>
          {tracks.length === 0 ? (
            <Text style={styles.emptyText}>No tracks yet.</Text>
          ) : (
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
                    <TouchableOpacity
                      style={styles.trackActionBtn}
                      onPress={() => handleAddToQueue(item)}
                    >
                      {justQueued[item.id] ? (
                        <Ionicons name="checkmark-circle" size={18} color="#10b981" />
                      ) : (
                        <Ionicons name="add-circle-outline" size={18} color="#9ca3af" />
                      )}
                    </TouchableOpacity>
                    <TouchableOpacity
                      style={styles.trackActionBtn}
                      onPress={() => handleToggleOffline(item)}
                    >
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
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0f131d', paddingTop: 40 },
  header: { paddingHorizontal: 16, marginBottom: 12 },
  headerTitle: { fontSize: 24, fontWeight: '700', color: '#f3f4f6', marginBottom: 12 },
  searchRow: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    paddingHorizontal: 12,
  },
  searchIcon: { marginRight: 8 },
  searchInput: {
    flex: 1,
    paddingVertical: 12,
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
  emptyText: { color: '#6b7280', fontSize: 13, marginBottom: 4 },
  errorState: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: 32,
    gap: 6,
  },
  errorTitle: { color: '#f3f4f6', fontSize: 16, fontWeight: '600', marginTop: 8 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  errorHint: { color: '#6b7280', fontSize: 12, textAlign: 'center', marginTop: 4 },
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
  albumCover: {
    width: '100%',
    aspectRatio: 1,
    borderRadius: 12,
    marginBottom: 8,
  },
  albumCoverPlaceholder: {
    width: '100%',
    aspectRatio: 1,
    borderRadius: 12,
    backgroundColor: ACCENT,
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
  trackArt: {
    width: 36,
    height: 36,
    borderRadius: 8,
    marginRight: 12,
  },
  trackText: { flex: 1 },
  trackTitle: { fontSize: 14, fontWeight: '600', color: '#f3f4f6' },
  trackSubtitle: { fontSize: 12, color: '#9ca3af', marginTop: 2 },
  trackActions: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  trackActionBtn: { padding: 6 },
});
