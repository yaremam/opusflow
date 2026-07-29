import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TouchableOpacity } from 'react-native';
import { audioPlayer, AudioPlayerState } from '../services/audioPlayer';

interface MiniPlayerBarProps {
  onOpenPlayer: () => void;
}

export function MiniPlayerBar({ onOpenPlayer }: MiniPlayerBarProps) {
  const [state, setState] = useState<AudioPlayerState>(audioPlayer.getState());

  useEffect(() => {
    return audioPlayer.subscribe((s) => setState(s));
  }, []);

  if (!state.currentTrack) return null;

  return (
    <TouchableOpacity style={styles.container} onPress={onOpenPlayer}>
      <View style={styles.infoRow}>
        <View style={styles.artPlaceholder}>
          <Text style={{ fontSize: 16 }}>🌌</Text>
        </View>
        <View style={styles.textCol}>
          <Text style={styles.title} numberOfLines={1}>
            {state.currentTrack.title}
          </Text>
          <Text style={styles.subtitle} numberOfLines={1}>
            {state.currentTrack.artistName}
          </Text>
        </View>
      </View>

      <View style={styles.controlsRow}>
        <TouchableOpacity
          onPress={(e) => {
            e.stopPropagation();
            audioPlayer.togglePlayPause();
          }}
        >
          <Text style={{ fontSize: 20 }}>{state.isPlaying ? '⏸️' : '▶️'}</Text>
        </TouchableOpacity>
        <TouchableOpacity
          onPress={(e) => {
            e.stopPropagation();
            audioPlayer.nextTrack();
          }}
        >
          <Text style={{ fontSize: 20, marginLeft: 12 }}>⏭️</Text>
        </TouchableOpacity>
      </View>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  container: {
    height: 52,
    backgroundColor: '#141824',
    borderTopWidth: 1,
    borderTopColor: 'rgba(255, 255, 255, 0.08)',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: 16,
  },
  infoRow: { flexDirection: 'row', alignItems: 'center', flex: 1 },
  artPlaceholder: {
    width: 34,
    height: 34,
    borderRadius: 8,
    backgroundColor: '#6366f1',
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 10,
  },
  textCol: { flex: 1 },
  title: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  subtitle: { fontSize: 11, color: '#9ca3af', marginTop: 1 },
  controlsRow: { flexDirection: 'row', alignItems: 'center' },
});
