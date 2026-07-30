import { Ionicons } from '@expo/vector-icons';
import { useState } from 'react';
import { FlatList, Image, StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { fetchAlbumsPage, Album, ListParams } from '../services/api';
import { useInfiniteList } from '../hooks/useInfiniteList';
import { LibraryFilterBar } from '../components/LibraryFilterBar';
import { ACCENT } from '../theme';

interface AlbumsListScreenProps {
  onOpenAlbum: (album: Album) => void;
}

// AlbumsListScreen is the Library hub's Albums tab (backlog/026 AC-1,
// AC-2) — a real infinite-scroll grid replacing the old "featured
// albums" strip (first page only, no search/sort/filter, issue #77).
// Tapping an album navigates to its detail screen rather than playing
// immediately, reversing PR #74's behavior now that AlbumDetailScreen
// exists (confirmed in grilling).
export function AlbumsListScreen({ onOpenAlbum }: AlbumsListScreenProps) {
  const [filters, setFilters] = useState<ListParams>({ sort: 'recent' });
  const { items, loading, loadingMore, error, hasMore, loadMore } = useInfiniteList(fetchAlbumsPage, filters);

  return (
    <View style={styles.container}>
      <LibraryFilterBar filters={filters} onChange={setFilters} searchPlaceholder="Search albums…" />

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
          <Text style={styles.emptyText}>No albums yet.</Text>
        </View>
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.id.toString()}
          numColumns={2}
          columnWrapperStyle={styles.row}
          contentContainerStyle={styles.listContent}
          onEndReached={loadMore}
          onEndReachedThreshold={0.4}
          ListFooterComponent={loadingMore ? <Text style={styles.footerText}>Loading more…</Text> : null}
          renderItem={({ item }) => (
            <TouchableOpacity style={styles.card} onPress={() => onOpenAlbum(item)}>
              {item.coverUrl ? (
                <Image source={{ uri: item.coverUrl }} style={styles.cover} />
              ) : (
                <View style={[styles.cover, styles.coverPlaceholder]}>
                  <Ionicons name="disc-outline" size={30} color="#ffffff" />
                </View>
              )}
              <Text style={styles.title} numberOfLines={1}>{item.title}</Text>
              <Text style={styles.sub} numberOfLines={1}>{item.artistName}</Text>
            </TouchableOpacity>
          )}
        />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, paddingHorizontal: 16 },
  listContent: { paddingTop: 12, paddingBottom: 100 },
  row: { gap: 12 },
  errorState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  emptyText: { color: '#6b7280', fontSize: 13 },
  footerText: { color: '#6b7280', fontSize: 12, textAlign: 'center', paddingVertical: 16 },
  card: { flex: 1, marginBottom: 16 },
  cover: { width: '100%', aspectRatio: 1, borderRadius: 12, marginBottom: 8 },
  coverPlaceholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
  title: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  sub: { fontSize: 11, color: '#9ca3af', marginTop: 2 },
});
