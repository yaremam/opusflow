import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TouchableOpacity, Image } from 'react-native';
import { audioPlayer, AudioPlayerState } from '../services/audioPlayer';
import { ACCENT } from '../theme';

interface MiniPlayerBarProps {
  onOpenPlayer: () => void;
}

export function MiniPlayerBar({ onOpenPlayer }: MiniPlayerBarProps) {
  const [state, setState] = useState<AudioPlayerState>(audioPlayer.getState());

  useEffect(() => {
    return audioPlayer.subscribe((s) => setState(s));
  }, []);

  if (!state.currentTrack) return null;
  const artUri = state.currentTrack.localCoverUrl || state.currentTrack.coverUrl;

  return (
    <TouchableOpacity style={styles.container} onPress={onOpenPlayer}>
      <View style={styles.infoRow}>
        {artUri ? (
          <Image source={{ uri: artUri }} style={styles.art} />
        ) : (
          <View style={styles.artPlaceholder}>
            <Ionicons name="disc-outline" size={18} color="#ffffff" />
          </View>
        )}
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
          <Ionicons name={state.isPlaying ? 'pause' : 'play'} size={20} color="#f3f4f6" />
        </TouchableOpacity>
        <TouchableOpacity
          onPress={(e) => {
            e.stopPropagation();
            audioPlayer.nextTrack();
          }}
        >
          <Ionicons name="play-skip-forward" size={20} color="#f3f4f6" style={{ marginLeft: 12 }} />
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
  art: {
    width: 34,
    height: 34,
    borderRadius: 8,
    marginRight: 10,
  },
  artPlaceholder: {
    width: 34,
    height: 34,
    borderRadius: 8,
    backgroundColor: ACCENT,
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 10,
  },
  textCol: { flex: 1 },
  title: { fontSize: 13, fontWeight: '600', color: '#f3f4f6' },
  subtitle: { fontSize: 11, color: '#9ca3af', marginTop: 1 },
  controlsRow: { flexDirection: 'row', alignItems: 'center' },
});
