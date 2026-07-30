import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { FlatList, Modal, StyleSheet, Text, TextInput, TouchableOpacity, View } from 'react-native';
import { createPlaylist, fetchPlaylistsPage, ListParams, Playlist } from '../services/api';
import { useInfiniteList } from '../hooks/useInfiniteList';
import { PlaylistCoverCollage } from '../components/PlaylistCoverCollage';
import { ACCENT, ACCENT_TINT_20 } from '../theme';

interface PlaylistsListScreenProps {
  onOpenPlaylist: (playlist: Playlist) => void;
}

// PlaylistsListScreen is the Library hub's 4th segment (backlog/028
// AC-1) — the same infinite-scroll grid pattern as AlbumsListScreen, with
// a "+ New playlist" card as the first tile and each cover a 2x2 collage
// instead of a single album image (AC-7).
export function PlaylistsListScreen({ onOpenPlaylist }: PlaylistsListScreenProps) {
  const [filters, setFilters] = useState<ListParams>({ sort: 'recent' });
  const { items, loading, loadingMore, error, hasMore, loadMore } = useInfiniteList(fetchPlaylistsPage, filters);

  const [creating, setCreating] = useState(false);
  const [name, setName] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  useEffect(() => {
    if (!creating) {
      setName('');
      setCreateError(null);
    }
  }, [creating]);

  async function handleCreate() {
    if (!name.trim()) return;
    setSubmitting(true);
    setCreateError(null);
    try {
      const playlist = await createPlaylist(name.trim());
      setCreating(false);
      onOpenPlaylist(playlist);
    } catch (e) {
      setCreateError(e instanceof Error ? e.message : 'Could not create that playlist.');
    } finally {
      setSubmitting(false);
    }
  }

  const gridItems: (Playlist | 'new')[] = ['new', ...items];

  return (
    <View style={styles.container}>
      <View style={styles.search}>
        <Ionicons name="search-outline" size={16} color="#6b7280" style={styles.searchIcon} />
        <TextInput
          style={styles.searchInput}
          placeholder="Search playlists…"
          placeholderTextColor="#6b7280"
          value={filters.q ?? ''}
          onChangeText={(q) => setFilters((prev) => ({ ...prev, q }))}
        />
      </View>
      <TouchableOpacity
        style={[styles.sortChip, filters.sort === 'name' && styles.sortChipOn]}
        onPress={() => setFilters((prev) => ({ ...prev, sort: prev.sort === 'name' ? 'recent' : 'name' }))}
      >
        <Text style={[styles.sortChipText, filters.sort === 'name' && styles.sortChipTextOn]}>
          Sort: {filters.sort === 'name' ? 'Name' : 'Recent'}
        </Text>
      </TouchableOpacity>

      {error ? (
        <View style={styles.errorState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : loading ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyText}>Loading…</Text>
        </View>
      ) : (
        <FlatList
          data={gridItems}
          keyExtractor={(item) => (item === 'new' ? 'new' : item.id.toString())}
          numColumns={2}
          columnWrapperStyle={styles.row}
          contentContainerStyle={styles.listContent}
          onEndReached={loadMore}
          onEndReachedThreshold={0.4}
          ListFooterComponent={loadingMore ? <Text style={styles.footerText}>Loading more…</Text> : null}
          renderItem={({ item }) =>
            item === 'new' ? (
              <TouchableOpacity style={styles.card} onPress={() => setCreating(true)}>
                <View style={styles.newCard}>
                  <Ionicons name="add" size={30} color="#6b7280" />
                </View>
                <Text style={styles.title}>New playlist</Text>
              </TouchableOpacity>
            ) : (
              <TouchableOpacity style={styles.card} onPress={() => onOpenPlaylist(item)}>
                <PlaylistCoverCollage coverUrls={item.coverUrls} style={styles.cover} />
                <Text style={styles.title} numberOfLines={1}>{item.name}</Text>
                <Text style={styles.sub} numberOfLines={1}>
                  {item.trackCount} song{item.trackCount === 1 ? '' : 's'}
                </Text>
              </TouchableOpacity>
            )
          }
        />
      )}

      <Modal visible={creating} transparent animationType="fade" onRequestClose={() => setCreating(false)}>
        <View style={styles.modalOverlay}>
          <View style={styles.modalCard}>
            <Text style={styles.modalTitle}>New playlist</Text>
            <TextInput
              style={styles.modalInput}
              placeholder="Playlist name"
              placeholderTextColor="#6b7280"
              value={name}
              onChangeText={setName}
              autoFocus
            />
            {createError && <Text style={styles.modalError}>{createError}</Text>}
            <View style={styles.modalActions}>
              <TouchableOpacity
                style={[styles.modalCreateBtn, (!name.trim() || submitting) && styles.modalCreateBtnDisabled]}
                onPress={handleCreate}
                disabled={!name.trim() || submitting}
              >
                <Text style={styles.modalCreateText}>{submitting ? 'Creating…' : 'Create playlist'}</Text>
              </TouchableOpacity>
              <TouchableOpacity style={styles.modalCancelBtn} onPress={() => setCreating(false)} disabled={submitting}>
                <Text style={styles.modalCancelText}>Cancel</Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, paddingHorizontal: 16 },
  search: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    paddingHorizontal: 12,
  },
  searchIcon: { marginRight: 8 },
  searchInput: { flex: 1, paddingVertical: 10, color: '#f3f4f6', fontSize: 14 },
  sortChip: {
    alignSelf: 'flex-start',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 6,
    marginTop: 8,
    marginBottom: 4,
  },
  sortChipOn: { borderColor: ACCENT, backgroundColor: ACCENT_TINT_20 },
  sortChipText: { fontSize: 11, fontWeight: '600', color: '#9ca3af' },
  sortChipTextOn: { color: ACCENT },
  listContent: { paddingTop: 12, paddingBottom: 100 },
  row: { gap: 12 },
  errorState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center' },
  emptyText: { color: '#6b7280', fontSize: 13 },
  footerText: { color: '#6b7280', fontSize: 12, textAlign: 'center', paddingVertical: 16 },
  card: { flex: 1, marginBottom: 16 },
  cover: { marginBottom: 8 },
  newCard: {
    width: '100%',
    aspectRatio: 1,
    borderRadius: 12,
    borderWidth: 1.5,
    borderStyle: 'dashed',
    borderColor: 'rgba(255, 255, 255, 0.12)',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 8,
  },
  title: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  sub: { fontSize: 11, color: '#9ca3af', marginTop: 2 },
  modalOverlay: { flex: 1, backgroundColor: 'rgba(5, 7, 10, 0.55)', alignItems: 'center', justifyContent: 'center', padding: 24 },
  modalCard: { width: '100%', maxWidth: 340, backgroundColor: '#141824', borderRadius: 16, borderWidth: 1, borderColor: 'rgba(255, 255, 255, 0.08)', padding: 20 },
  modalTitle: { fontSize: 16, fontWeight: '700', color: '#f3f4f6', marginBottom: 14 },
  modalInput: { backgroundColor: '#0f131d', borderWidth: 1, borderColor: 'rgba(255, 255, 255, 0.08)', borderRadius: 10, padding: 12, color: '#f3f4f6', fontSize: 15 },
  modalError: { color: '#f87171', fontSize: 12, marginTop: 8 },
  modalActions: { marginTop: 16, gap: 8 },
  modalCreateBtn: { backgroundColor: ACCENT, borderRadius: 10, paddingVertical: 12, alignItems: 'center' },
  modalCreateBtnDisabled: { backgroundColor: '#2a3040' },
  modalCreateText: { color: '#0a1512', fontSize: 14, fontWeight: '700' },
  modalCancelBtn: { paddingVertical: 10, alignItems: 'center' },
  modalCancelText: { color: '#9ca3af', fontSize: 13, fontWeight: '600' },
});
