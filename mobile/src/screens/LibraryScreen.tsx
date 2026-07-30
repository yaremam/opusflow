import { useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { Album, Artist, Playlist } from '../services/api';
import { ArtistsListScreen } from './ArtistsListScreen';
import { AlbumsListScreen } from './AlbumsListScreen';
import { SongsListScreen } from './SongsListScreen';
import { PlaylistsListScreen } from './PlaylistsListScreen';
import { ArtistDetailScreen } from './ArtistDetailScreen';
import { AlbumDetailScreen } from './AlbumDetailScreen';
import { PlaylistDetailScreen } from './PlaylistDetailScreen';
import { ACCENT } from '../theme';

interface LibraryScreenProps {
  onOpenPlayer?: () => void;
}

type LibraryTab = 'artists' | 'albums' | 'songs' | 'playlists';
type LibraryStack =
  | { view: 'hub' }
  | { view: 'artist'; artist: Artist }
  | { view: 'album'; album: Album }
  | { view: 'playlist'; playlist: Playlist };

// LibraryScreen is the Library tab's hub (backlog/026 AC-1) — a segmented
// control over real infinite-scroll lists (Artists/Albums/Songs,
// replacing the old single screen's "featured albums" strip + unfiltered
// track dump from issue #77; Playlists joined as a 4th segment in
// backlog/028). Detail navigation is a small hand-rolled stack (AC-6)
// rather than a navigation library — this component unmounting whenever
// the bottom tab bar switches away naturally resets it back to the hub,
// since its state is just a plain useState.
export function LibraryScreen({ onOpenPlayer }: LibraryScreenProps) {
  const [activeTab, setActiveTab] = useState<LibraryTab>('artists');
  const [stack, setStack] = useState<LibraryStack>({ view: 'hub' });

  if (stack.view === 'artist') {
    return (
      <ArtistDetailScreen
        artistId={stack.artist.id}
        onBack={() => setStack({ view: 'hub' })}
        onOpenAlbum={(album) => setStack({ view: 'album', album })}
      />
    );
  }

  if (stack.view === 'album') {
    return (
      <AlbumDetailScreen
        album={stack.album}
        onBack={() => setStack({ view: 'hub' })}
        onOpenPlayer={onOpenPlayer}
      />
    );
  }

  if (stack.view === 'playlist') {
    return (
      <PlaylistDetailScreen
        playlist={stack.playlist}
        onBack={() => setStack({ view: 'hub' })}
        onDeleted={() => setStack({ view: 'hub' })}
        onOpenPlayer={onOpenPlayer}
      />
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Library</Text>
      </View>

      <View style={styles.segmented}>
        {(['artists', 'albums', 'songs', 'playlists'] as const).map((tab) => (
          <TouchableOpacity
            key={tab}
            style={[styles.segment, activeTab === tab && styles.segmentActive]}
            onPress={() => setActiveTab(tab)}
          >
            <Text style={[styles.segmentText, activeTab === tab && styles.segmentTextActive]}>
              {tab === 'artists' ? 'Artists' : tab === 'albums' ? 'Albums' : tab === 'songs' ? 'Songs' : 'Playlists'}
            </Text>
          </TouchableOpacity>
        ))}
      </View>

      {activeTab === 'artists' && (
        <ArtistsListScreen onOpenArtist={(artist) => setStack({ view: 'artist', artist })} />
      )}
      {activeTab === 'albums' && (
        <AlbumsListScreen onOpenAlbum={(album) => setStack({ view: 'album', album })} />
      )}
      {activeTab === 'songs' && <SongsListScreen onOpenPlayer={onOpenPlayer} />}
      {activeTab === 'playlists' && (
        <PlaylistsListScreen onOpenPlaylist={(playlist) => setStack({ view: 'playlist', playlist })} />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0f131d', paddingTop: 40 },
  header: { paddingHorizontal: 16, marginBottom: 12 },
  headerTitle: { fontSize: 24, fontWeight: '700', color: '#f3f4f6' },
  segmented: {
    flexDirection: 'row',
    backgroundColor: '#141824',
    borderRadius: 10,
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    padding: 3,
    marginHorizontal: 16,
    marginBottom: 12,
  },
  segment: { flex: 1, alignItems: 'center', paddingVertical: 7, borderRadius: 8 },
  segmentActive: { backgroundColor: ACCENT },
  segmentText: { fontSize: 12, fontWeight: '600', color: '#9ca3af' },
  segmentTextActive: { color: '#0a1512' },
});
