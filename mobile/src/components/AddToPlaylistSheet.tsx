import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { FlatList, Modal, StyleSheet, Text, TextInput, TouchableOpacity, View } from 'react-native';
import {
  addTrackToPlaylist,
  createPlaylist,
  fetchPlaylistsContainingTrack,
  fetchPlaylistsPage,
  Playlist,
} from '../services/api';
import { ACCENT, ACCENT_TINT_20 } from '../theme';

interface AddToPlaylistSheetProps {
  visible: boolean;
  trackId: number | null;
  trackTitle: string;
  onClose: () => void;
}

// AddToPlaylistSheet is every track row's long-press destination (AC-5,
// backlog/028) — a bottom sheet with a checkable list of existing
// playlists plus an inline "+ New playlist" row that creates and adds in
// one step, matching web's AddToPlaylistMenu. Opened straight from a
// long-press rather than an intermediate action sheet, since "Add to
// playlist" is the only action this exposes that a row doesn't already
// have a dedicated icon for.
export function AddToPlaylistSheet({ visible, trackId, trackTitle, onClose }: AddToPlaylistSheetProps) {
  const [playlists, setPlaylists] = useState<Playlist[]>([]);
  const [containingIds, setContainingIds] = useState<Set<number>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [addingId, setAddingId] = useState<number | null>(null);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [createSubmitting, setCreateSubmitting] = useState(false);

  useEffect(() => {
    if (!visible || trackId === null) return;
    setLoading(true);
    setError(null);
    setCreating(false);
    setNewName('');
    let cancelled = false;
    Promise.all([fetchPlaylistsPage({ sort: 'name', page: 1 }), fetchPlaylistsContainingTrack(trackId)])
      .then(([page, containing]) => {
        if (cancelled) return;
        setPlaylists(page.items);
        setContainingIds(new Set(containing.map((p) => p.id)));
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Could not load your playlists.');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [visible, trackId]);

  async function handleAdd(playlist: Playlist) {
    if (trackId === null) return;
    setAddingId(playlist.id);
    try {
      await addTrackToPlaylist(playlist.id, trackId);
      setContainingIds((prev) => new Set(prev).add(playlist.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not add this track.');
    } finally {
      setAddingId(null);
    }
  }

  async function handleCreate() {
    if (trackId === null || !newName.trim()) return;
    setCreateSubmitting(true);
    try {
      const playlist = await createPlaylist(newName.trim());
      await addTrackToPlaylist(playlist.id, trackId);
      setPlaylists((prev) => [playlist, ...prev]);
      setContainingIds((prev) => new Set(prev).add(playlist.id));
      setNewName('');
      setCreating(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not create that playlist.');
    } finally {
      setCreateSubmitting(false);
    }
  }

  return (
    <Modal visible={visible} transparent animationType="slide" onRequestClose={onClose}>
      <TouchableOpacity style={styles.overlay} activeOpacity={1} onPress={onClose}>
        <TouchableOpacity style={styles.sheet} activeOpacity={1} onPress={() => {}}>
          <View style={styles.handle} />
          <Text style={styles.title} numberOfLines={1}>
            Add "{trackTitle}" to playlist
          </Text>

          {creating ? (
            <View style={styles.newForm}>
              <TextInput
                style={styles.newInput}
                placeholder="Playlist name"
                placeholderTextColor="#6b7280"
                value={newName}
                onChangeText={setNewName}
                autoFocus
              />
              <View style={styles.newActions}>
                <TouchableOpacity style={styles.newCancelBtn} onPress={() => setCreating(false)} disabled={createSubmitting}>
                  <Text style={styles.newCancelText}>Cancel</Text>
                </TouchableOpacity>
                <TouchableOpacity
                  style={[styles.newCreateBtn, (!newName.trim() || createSubmitting) && styles.newCreateBtnDisabled]}
                  onPress={handleCreate}
                  disabled={!newName.trim() || createSubmitting}
                >
                  <Text style={styles.newCreateText}>{createSubmitting ? 'Creating…' : 'Create & add'}</Text>
                </TouchableOpacity>
              </View>
            </View>
          ) : (
            <TouchableOpacity style={styles.newRow} onPress={() => setCreating(true)}>
              <View style={styles.newPlus}>
                <Ionicons name="add" size={16} color={ACCENT} />
              </View>
              <Text style={styles.newRowText}>New playlist…</Text>
            </TouchableOpacity>
          )}

          {error && <Text style={styles.errorText}>{error}</Text>}

          {loading ? (
            <Text style={styles.emptyText}>Loading…</Text>
          ) : !error && playlists.length === 0 && !creating ? (
            <Text style={styles.emptyText}>No playlists yet — create one above.</Text>
          ) : (
            <FlatList
              data={playlists}
              keyExtractor={(item) => item.id.toString()}
              style={styles.list}
              renderItem={({ item }) => {
                const inPlaylist = containingIds.has(item.id);
                return (
                  <TouchableOpacity
                    style={styles.row}
                    onPress={() => handleAdd(item)}
                    disabled={inPlaylist || addingId === item.id}
                  >
                    <Text style={styles.rowText} numberOfLines={1}>
                      {item.name}
                    </Text>
                    <View style={[styles.check, inPlaylist && styles.checkOn]}>
                      {inPlaylist && <Ionicons name="checkmark" size={13} color="#0a1512" />}
                    </View>
                  </TouchableOpacity>
                );
              }}
            />
          )}
        </TouchableOpacity>
      </TouchableOpacity>
    </Modal>
  );
}

const styles = StyleSheet.create({
  overlay: { flex: 1, backgroundColor: 'rgba(5, 7, 10, 0.55)', justifyContent: 'flex-end' },
  sheet: {
    backgroundColor: '#141824',
    borderTopLeftRadius: 18,
    borderTopRightRadius: 18,
    borderTopWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    paddingTop: 10,
    paddingBottom: 32,
    paddingHorizontal: 20,
    maxHeight: '78%',
  },
  handle: { width: 32, height: 4, borderRadius: 2, backgroundColor: 'rgba(255, 255, 255, 0.08)', alignSelf: 'center', marginBottom: 12 },
  title: { fontSize: 14, fontWeight: '700', color: '#f3f4f6', marginBottom: 10 },
  newRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    paddingVertical: 10,
    borderBottomWidth: 1,
    borderBottomColor: 'rgba(255, 255, 255, 0.08)',
    marginBottom: 6,
  },
  newPlus: {
    width: 30,
    height: 30,
    borderRadius: 15,
    backgroundColor: ACCENT_TINT_20,
    borderWidth: 1,
    borderColor: ACCENT,
    borderStyle: 'dashed',
    alignItems: 'center',
    justifyContent: 'center',
  },
  newRowText: { fontSize: 13, fontWeight: '600', color: ACCENT },
  newForm: { borderBottomWidth: 1, borderBottomColor: 'rgba(255, 255, 255, 0.08)', paddingBottom: 12, marginBottom: 6, gap: 10 },
  newInput: {
    backgroundColor: '#0f131d',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 10,
    padding: 12,
    color: '#f3f4f6',
    fontSize: 14,
  },
  newActions: { flexDirection: 'row', justifyContent: 'flex-end', gap: 10 },
  newCancelBtn: { paddingVertical: 8, paddingHorizontal: 12 },
  newCancelText: { color: '#9ca3af', fontSize: 13, fontWeight: '600' },
  newCreateBtn: { backgroundColor: ACCENT, borderRadius: 8, paddingVertical: 8, paddingHorizontal: 14 },
  newCreateBtnDisabled: { backgroundColor: '#2a3040' },
  newCreateText: { color: '#0a1512', fontSize: 13, fontWeight: '700' },
  errorText: { color: '#f87171', fontSize: 12, marginBottom: 8 },
  emptyText: { color: '#6b7280', fontSize: 13, paddingVertical: 16, textAlign: 'center' },
  list: { flexGrow: 0 },
  row: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingVertical: 10 },
  rowText: { flex: 1, fontSize: 14, color: '#f3f4f6' },
  check: { width: 20, height: 20, borderRadius: 5, borderWidth: 1.5, borderColor: 'rgba(255, 255, 255, 0.08)', alignItems: 'center', justifyContent: 'center' },
  checkOn: { backgroundColor: ACCENT, borderColor: ACCENT },
});
