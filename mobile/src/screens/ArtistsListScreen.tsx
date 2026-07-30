import { Ionicons } from '@expo/vector-icons';
import { useState } from 'react';
import { FlatList, Image, StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { fetchArtistsPage, Artist, ListParams } from '../services/api';
import { useInfiniteList } from '../hooks/useInfiniteList';
import { LibraryFilterBar } from '../components/LibraryFilterBar';
import { ACCENT } from '../theme';

interface ArtistsListScreenProps {
  onOpenArtist: (artist: Artist) => void;
}

// ArtistsListScreen is the Library hub's Artists tab (backlog/026 AC-1,
// AC-2) — real infinite-scroll + search/sort/genre/year, replacing the
// old screen's total lack of any artist browsing at all (issue #77).
export function ArtistsListScreen({ onOpenArtist }: ArtistsListScreenProps) {
  const [filters, setFilters] = useState<ListParams>({ sort: 'recent' });
  const { items, loading, loadingMore, error, hasMore, loadMore } = useInfiniteList(fetchArtistsPage, filters);

  return (
    <View style={styles.container}>
      <LibraryFilterBar filters={filters} onChange={setFilters} searchPlaceholder="Search artists…" />

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
          <Text style={styles.emptyText}>No artists yet.</Text>
        </View>
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.id.toString()}
          contentContainerStyle={styles.listContent}
          onEndReached={loadMore}
          onEndReachedThreshold={0.4}
          ListFooterComponent={loadingMore ? <Text style={styles.footerText}>Loading more…</Text> : null}
          renderItem={({ item }) => (
            <TouchableOpacity style={styles.row} onPress={() => onOpenArtist(item)}>
              {item.photoUrl ? (
                <Image source={{ uri: item.photoUrl }} style={styles.avatar} />
              ) : (
                <View style={[styles.avatar, styles.avatarPlaceholder]}>
                  <Text style={styles.avatarInitial}>{item.name.charAt(0).toUpperCase()}</Text>
                </View>
              )}
              <View style={styles.rowText}>
                <Text style={styles.name}>{item.name}</Text>
                <Text style={styles.sub}>{item.albumCount} {item.albumCount === 1 ? 'album' : 'albums'}</Text>
              </View>
              <Ionicons name="chevron-forward" size={18} color="#6b7280" />
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
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: 'rgba(255, 255, 255, 0.08)',
  },
  avatar: { width: 42, height: 42, borderRadius: 21 },
  avatarPlaceholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
  avatarInitial: { fontSize: 15, fontWeight: '700', color: '#06231f' },
  rowText: { flex: 1 },
  name: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  sub: { fontSize: 10.5, color: '#9ca3af', marginTop: 1 },
});
