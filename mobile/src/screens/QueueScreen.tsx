import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import DraggableFlatList, { type RenderItemParams } from 'react-native-draggable-flatlist';
import { audioPlayer, AudioPlayerState } from '../services/audioPlayer';
import { Track } from '../services/api';
import { ACCENT, ACCENT_TINT_20 } from '../theme';

interface QueueScreenProps {
  onClose: () => void;
}

// QueueScreen is the Player screen's top-right button destination
// (backlog/027 AC-1) — full parity with web's Queue Drawer: tap a track
// to jump to it (AC-2), remove one (AC-3), and drag to reorder (AC-4) via
// react-native-draggable-flatlist, since hand-rolling drag/auto-scroll/
// virtualization on React Native risks a janky reorder for a feature
// whose entire value is feeling fluid.
export function QueueScreen({ onClose }: QueueScreenProps) {
  const [playerState, setPlayerState] = useState<AudioPlayerState>(audioPlayer.getState());

  useEffect(() => {
    return audioPlayer.subscribe((state) => setPlayerState(state));
  }, []);

  function renderItem({ item, getIndex, drag, isActive }: RenderItemParams<Track>) {
    const index = getIndex();
    const isCurrent = index === playerState.queueIndex;

    return (
      <TouchableOpacity
        style={[styles.row, isCurrent && styles.rowCurrent, isActive && styles.rowActive]}
        onPress={() => {
          if (!isCurrent && index !== undefined) audioPlayer.jumpTo(index);
        }}
        onLongPress={drag}
        disabled={isCurrent}
      >
        <TouchableOpacity onLongPress={drag} hitSlop={8}>
          <Ionicons name="reorder-three-outline" size={20} color="#6b7280" />
        </TouchableOpacity>

        <View style={styles.rowIcon}>
          <Ionicons name="musical-notes-outline" size={16} color="#9ca3af" />
        </View>

        <View style={styles.rowText}>
          <Text style={[styles.rowTitle, isCurrent && styles.rowTitleCurrent]} numberOfLines={1}>
            {item.title}
          </Text>
          {isCurrent ? (
            <Text style={styles.nowPlayingTag}>NOW PLAYING</Text>
          ) : (
            <Text style={styles.rowSubtitle} numberOfLines={1}>
              {item.artistName}
            </Text>
          )}
        </View>

        {!isCurrent && (
          <TouchableOpacity
            style={styles.removeBtn}
            onPress={() => index !== undefined && audioPlayer.removeFromQueue(index)}
            hitSlop={8}
          >
            <Ionicons name="close" size={18} color="#6b7280" />
          </TouchableOpacity>
        )}
      </TouchableOpacity>
    );
  }

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onClose}>
          <Ionicons name="chevron-back" size={18} color="#f3f4f6" />
        </TouchableOpacity>
        <Text style={styles.topTitle}>UP NEXT</Text>
        <View style={styles.iconBtn} />
      </View>

      {playerState.queue.length === 0 ? (
        <View style={styles.emptyState}>
          <Ionicons name="list-outline" size={40} color="#3a4150" />
          <Text style={styles.emptyText}>Nothing queued.</Text>
        </View>
      ) : (
        <DraggableFlatList
          data={playerState.queue}
          keyExtractor={(item, index) => `${item.id}-${index}`}
          renderItem={renderItem}
          onDragEnd={({ from, to }) => audioPlayer.reorderQueue(from, to)}
          contentContainerStyle={styles.listContent}
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
    paddingHorizontal: 24,
    marginBottom: 16,
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
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 8 },
  emptyText: { color: '#6b7280', fontSize: 14 },
  listContent: { paddingHorizontal: 16, paddingBottom: 40 },
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 10,
    marginBottom: 8,
  },
  rowCurrent: {
    backgroundColor: ACCENT_TINT_20,
    borderColor: ACCENT,
  },
  rowActive: {
    borderColor: ACCENT,
    opacity: 0.85,
  },
  rowIcon: {
    width: 32,
    height: 32,
    borderRadius: 8,
    backgroundColor: 'rgba(255, 255, 255, 0.05)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  rowText: { flex: 1 },
  rowTitle: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  rowTitleCurrent: { color: ACCENT },
  rowSubtitle: { fontSize: 11, color: '#9ca3af', marginTop: 1 },
  nowPlayingTag: { fontSize: 10, fontWeight: '700', color: ACCENT, letterSpacing: 0.4, marginTop: 1 },
  removeBtn: { padding: 4 },
});
