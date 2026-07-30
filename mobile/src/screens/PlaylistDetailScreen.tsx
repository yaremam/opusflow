import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { Modal, StyleSheet, Text, TextInput, TouchableOpacity, View } from 'react-native';
import DraggableFlatList, { type RenderItemParams } from 'react-native-draggable-flatlist';
import {
  deletePlaylist,
  fetchPlaylistDetail,
  Playlist,
  PlaylistDetail,
  PlaylistTrack,
  removePlaylistTrack,
  renamePlaylist,
  reorderPlaylistTracks,
} from '../services/api';
import { audioPlayer } from '../services/audioPlayer';
import { AddToPlaylistSheet } from '../components/AddToPlaylistSheet';
import { PlaylistCoverCollage } from '../components/PlaylistCoverCollage';
import { ACCENT } from '../theme';

interface PlaylistDetailScreenProps {
  playlist: Playlist;
  onBack: () => void;
  onDeleted: () => void;
  onOpenPlayer?: () => void;
}

// formatDuration renders a track's own length as "m:ss" (AlbumDetailScreen's
// convention), but the playlist total this also renders can run well past
// an hour, so it grows to "h:mm:ss" past 3600s the same way web's
// formatDuration does (web/src/api/library.ts).
function formatDuration(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600);
  const m = Math.floor((totalSeconds % 3600) / 60);
  const s = totalSeconds % 60;
  const mm = h > 0 ? String(m).padStart(2, '0') : String(m);
  const ss = String(s).padStart(2, '0');
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`;
}

// PlaylistDetailScreen matches web's PlaylistDetailPage (backlog/028
// AC-3/AC-4): tap the title to rename, a top-bar delete button that
// opens a confirm dialog (matching web's single "Delete playlist…"
// button — no intermediate action sheet for what's the only action
// there), drag-to-reorder via react-native-draggable-flatlist (already
// QueueScreen's own reorder mechanism), and per-track play/add-to-queue/
// add-to-playlist/remove — persisted to the backend, not the in-memory
// playback queue.
export function PlaylistDetailScreen({ playlist, onBack, onDeleted, onOpenPlayer }: PlaylistDetailScreenProps) {
  const [detail, setDetail] = useState<PlaylistDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rowError, setRowError] = useState<string | null>(null);

  const [renaming, setRenaming] = useState(false);
  const [renameValue, setRenameValue] = useState('');
  const [renameSubmitting, setRenameSubmitting] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [deleteSubmitting, setDeleteSubmitting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const [addSheet, setAddSheet] = useState<{ trackId: number; trackTitle: string } | null>(null);
  const [justQueued, setJustQueued] = useState<Record<number, boolean>>({});

  function load() {
    setLoading(true);
    setError(null);
    fetchPlaylistDetail(playlist.id)
      .then(setDetail)
      .catch((e) => setError(e instanceof Error ? e.message : 'Could not load this playlist.'))
      .finally(() => setLoading(false));
  }

  useEffect(load, [playlist.id]);

  function handlePlayTrack(index: number) {
    if (!detail) return;
    audioPlayer.playQueue(detail.tracks, index);
    onOpenPlayer?.();
  }

  function handleAddToQueue(track: PlaylistTrack) {
    audioPlayer.addToQueue(track);
    setJustQueued((prev) => ({ ...prev, [track.playlistTrackId]: true }));
    setTimeout(() => setJustQueued((prev) => ({ ...prev, [track.playlistTrackId]: false })), 1500);
  }

  async function handleRemoveTrack(playlistTrackId: number) {
    setRowError(null);
    try {
      setDetail(await removePlaylistTrack(playlist.id, playlistTrackId));
    } catch (e) {
      setRowError(e instanceof Error ? e.message : 'Could not remove that track.');
    }
  }

  async function handleDragEnd(from: number, to: number) {
    if (!detail || from === to) return;
    const track = detail.tracks[from];
    setRowError(null);
    try {
      const updated = await reorderPlaylistTracks(playlist.id, track.playlistTrackId, to);
      setDetail(updated);
    } catch (e) {
      setRowError(e instanceof Error ? e.message : 'Could not reorder that track.');
      load();
    }
  }

  function startRename() {
    if (!detail) return;
    setRenameValue(detail.name);
    setRenaming(true);
  }

  async function commitRename() {
    const trimmed = renameValue.trim();
    if (!detail || !trimmed || trimmed === detail.name) {
      setRenaming(false);
      return;
    }
    setRenameSubmitting(true);
    try {
      setDetail(await renamePlaylist(playlist.id, trimmed));
      setRenaming(false);
    } catch (e) {
      setRowError(e instanceof Error ? e.message : 'Could not rename this playlist.');
    } finally {
      setRenameSubmitting(false);
    }
  }

  async function confirmDelete() {
    setDeleteSubmitting(true);
    setDeleteError(null);
    try {
      await deletePlaylist(playlist.id);
      onDeleted();
    } catch (e) {
      setDeleteError(e instanceof Error ? e.message : 'Could not delete this playlist.');
      setDeleteSubmitting(false);
    }
  }

  function renderItem({ item, getIndex, drag, isActive }: RenderItemParams<PlaylistTrack>) {
    const index = getIndex();
    return (
      <TouchableOpacity
        style={[styles.trackRow, isActive && styles.trackRowActive]}
        onPress={() => index !== undefined && handlePlayTrack(index)}
        onLongPress={() => setAddSheet({ trackId: item.id, trackTitle: item.title })}
      >
        <TouchableOpacity onLongPress={drag} hitSlop={8}>
          <Ionicons name="reorder-three-outline" size={18} color="#6b7280" />
        </TouchableOpacity>

        <View style={styles.trackIcon}>
          <Ionicons name="musical-notes-outline" size={14} color="#9ca3af" />
        </View>

        <View style={styles.trackText}>
          <Text style={styles.trackTitle} numberOfLines={1}>{item.title}</Text>
          <Text style={styles.trackSubtitle} numberOfLines={1}>{item.artistName}</Text>
        </View>

        <Text style={styles.trackDur}>{formatDuration(item.durationSeconds)}</Text>

        <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleAddToQueue(item)}>
          {justQueued[item.playlistTrackId] ? (
            <Ionicons name="checkmark-circle" size={16} color="#10b981" />
          ) : (
            <Ionicons name="add-circle-outline" size={16} color="#9ca3af" />
          )}
        </TouchableOpacity>
        <TouchableOpacity style={styles.trackActionBtn} onPress={() => handleRemoveTrack(item.playlistTrackId)}>
          <Ionicons name="close" size={16} color="#9ca3af" />
        </TouchableOpacity>
      </TouchableOpacity>
    );
  }

  const totalSeconds = detail ? detail.tracks.reduce((sum, t) => sum + t.durationSeconds, 0) : 0;

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onBack}>
          <Ionicons name="chevron-back" size={18} color="#f3f4f6" />
        </TouchableOpacity>
        <Text style={styles.topTitle}>PLAYLIST</Text>
        <TouchableOpacity style={styles.iconBtn} onPress={() => setDeleting(true)}>
          <Ionicons name="trash-outline" size={18} color="#f87171" />
        </TouchableOpacity>
      </View>

      {error ? (
        <View style={styles.emptyState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : loading || !detail ? (
        <Text style={styles.emptyText}>Loading…</Text>
      ) : (
        <>
          <View style={styles.hero}>
            <PlaylistCoverCollage coverUrls={detail.coverUrls} size={88} />
            <View style={styles.heroText}>
              {renaming ? (
                <TextInput
                  style={styles.renameInput}
                  value={renameValue}
                  onChangeText={setRenameValue}
                  onBlur={commitRename}
                  onSubmitEditing={commitRename}
                  editable={!renameSubmitting}
                  autoFocus
                />
              ) : (
                <TouchableOpacity onPress={startRename}>
                  <Text style={styles.heroTitle} numberOfLines={2}>{detail.name}</Text>
                </TouchableOpacity>
              )}
              <Text style={styles.heroSub}>
                {detail.trackCount} song{detail.trackCount === 1 ? '' : 's'}
                {totalSeconds > 0 ? ` · ${formatDuration(totalSeconds)}` : ''}
              </Text>
            </View>
          </View>

          {rowError && <Text style={styles.rowErrorText}>{rowError}</Text>}

          {detail.tracks.length === 0 ? (
            <View style={styles.emptyState}>
              <Ionicons name="musical-notes-outline" size={28} color="#3a4150" />
              <Text style={styles.emptyText}>No songs yet — add some from Songs or an album.</Text>
            </View>
          ) : (
            <DraggableFlatList
              data={detail.tracks}
              keyExtractor={(item) => item.playlistTrackId.toString()}
              renderItem={renderItem}
              onDragEnd={({ from, to }) => handleDragEnd(from, to)}
              contentContainerStyle={styles.listContent}
            />
          )}
        </>
      )}

      <Modal visible={deleting} transparent animationType="fade" onRequestClose={() => setDeleting(false)}>
        <View style={styles.modalOverlay}>
          <View style={styles.modalCard}>
            <Text style={styles.modalTitle}>Delete "{detail?.name}"?</Text>
            <Text style={styles.modalBody}>
              This removes the playlist and its track order. The songs themselves stay in your library.
            </Text>
            {deleteError && <Text style={styles.modalError}>{deleteError}</Text>}
            <View style={styles.modalActions}>
              <TouchableOpacity
                style={[styles.modalDeleteBtn, deleteSubmitting && styles.modalCreateBtnDisabled]}
                onPress={confirmDelete}
                disabled={deleteSubmitting}
              >
                <Text style={styles.modalDeleteText}>{deleteSubmitting ? 'Deleting…' : 'Delete playlist'}</Text>
              </TouchableOpacity>
              <TouchableOpacity style={styles.modalCancelBtn} onPress={() => setDeleting(false)} disabled={deleteSubmitting}>
                <Text style={styles.modalCancelText}>Cancel</Text>
              </TouchableOpacity>
            </View>
          </View>
        </View>
      </Modal>

      <AddToPlaylistSheet
        visible={addSheet !== null}
        trackId={addSheet?.trackId ?? null}
        trackTitle={addSheet?.trackTitle ?? ''}
        onClose={() => setAddSheet(null)}
      />
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
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 8, paddingHorizontal: 32 },
  emptyText: { color: '#6b7280', fontSize: 13, textAlign: 'center', marginTop: 20 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  rowErrorText: { color: '#f87171', fontSize: 12, paddingHorizontal: 16, marginBottom: 8 },
  hero: { flexDirection: 'row', gap: 12, marginBottom: 14, alignItems: 'flex-end', paddingHorizontal: 16 },
  heroText: { flex: 1 },
  heroTitle: { fontSize: 18, fontWeight: '700', color: '#f3f4f6' },
  heroSub: { fontSize: 11, color: '#9ca3af', marginTop: 4 },
  renameInput: {
    fontSize: 18,
    fontWeight: '700',
    color: '#f3f4f6',
    borderBottomWidth: 1,
    borderBottomColor: ACCENT,
    paddingVertical: 2,
  },
  listContent: { paddingHorizontal: 16, paddingBottom: 100 },
  trackRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 10,
    marginBottom: 8,
  },
  trackRowActive: { borderColor: ACCENT, opacity: 0.85 },
  trackIcon: {
    width: 30,
    height: 30,
    borderRadius: 7,
    backgroundColor: 'rgba(255, 255, 255, 0.05)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  trackText: { flex: 1 },
  trackTitle: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  trackSubtitle: { fontSize: 11, color: '#9ca3af', marginTop: 1 },
  trackDur: { fontSize: 11, color: '#9ca3af' },
  trackActionBtn: { padding: 4 },
  modalOverlay: { flex: 1, backgroundColor: 'rgba(5, 7, 10, 0.55)', alignItems: 'center', justifyContent: 'center', padding: 24 },
  modalCard: { width: '100%', maxWidth: 340, backgroundColor: '#141824', borderRadius: 16, borderWidth: 1, borderColor: 'rgba(255, 255, 255, 0.08)', padding: 20 },
  modalTitle: { fontSize: 16, fontWeight: '700', color: '#f3f4f6', marginBottom: 8 },
  modalBody: { fontSize: 13, color: '#9ca3af', lineHeight: 18 },
  modalError: { color: '#f87171', fontSize: 12, marginTop: 8 },
  modalActions: { marginTop: 16, gap: 8 },
  modalDeleteBtn: { backgroundColor: 'rgba(239, 68, 68, 0.15)', borderWidth: 1, borderColor: 'rgba(239, 68, 68, 0.3)', borderRadius: 10, paddingVertical: 12, alignItems: 'center' },
  modalCreateBtnDisabled: { opacity: 0.6 },
  modalDeleteText: { color: '#f87171', fontSize: 14, fontWeight: '700' },
  modalCancelBtn: { paddingVertical: 10, alignItems: 'center' },
  modalCancelText: { color: '#9ca3af', fontSize: 13, fontWeight: '600' },
});
