import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TouchableOpacity, Image } from 'react-native';
import { audioPlayer, AudioPlayerState } from '../services/audioPlayer';
import { ACCENT, ACCENT_TINT_20 } from '../theme';

interface PlayerScreenProps {
  onMinimize?: () => void;
}

export function PlayerScreen({ onMinimize }: PlayerScreenProps) {
  const [playerState, setPlayerState] = useState<AudioPlayerState>(
    audioPlayer.getState()
  );

  useEffect(() => {
    return audioPlayer.subscribe((state) => setPlayerState(state));
  }, []);

  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
  };

  const track = playerState.currentTrack;

  if (!track) {
    return (
      <View style={styles.container}>
        <View style={styles.topBar}>
          <TouchableOpacity style={styles.iconBtn} onPress={onMinimize}>
            <Ionicons name="chevron-down" size={18} color="#f3f4f6" />
          </TouchableOpacity>
          <Text style={styles.topTitle}>NOW PLAYING</Text>
          <View style={styles.iconBtn} />
        </View>
        <View style={styles.emptyState}>
          <Ionicons name="disc-outline" size={56} color="#3a4150" />
          <Text style={styles.emptyTitle}>Nothing playing</Text>
          <Text style={styles.emptyText}>Pick a track from your library to start listening.</Text>
        </View>
      </View>
    );
  }

  const activeArtworkUri = track.localCoverUrl || track.coverUrl;

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onMinimize}>
          <Ionicons name="chevron-down" size={18} color="#f3f4f6" />
        </TouchableOpacity>
        <Text style={styles.topTitle}>NOW PLAYING</Text>
        <TouchableOpacity style={styles.iconBtn}>
          <Ionicons name="list-outline" size={18} color="#f3f4f6" />
        </TouchableOpacity>
      </View>

      <View style={styles.artworkContainer}>
        {activeArtworkUri ? (
          <Image
            source={{ uri: activeArtworkUri }}
            style={styles.artworkImage}
            resizeMode="cover"
          />
        ) : (
          <View style={styles.artworkPlaceholder}>
            <Ionicons name="disc-outline" size={64} color="#ffffff" />
          </View>
        )}
      </View>

      <View style={styles.trackDetails}>
        <Text style={styles.trackTitle}>{track.title}</Text>
        <Text style={styles.trackSubtitle}>
          {track.artistName} — {track.albumTitle}
        </Text>
        <View style={styles.qualityBadge}>
          <Ionicons
            name={track.localCoverUrl ? 'download-outline' : 'wifi-outline'}
            size={12}
            color={ACCENT}
          />
          <Text style={styles.qualityText}>
            {track.localCoverUrl ? 'Offline' : 'Streaming'}
          </Text>
        </View>
      </View>

      <View style={styles.progressContainer}>
        <View style={styles.progressBar}>
          <View style={[styles.progressFill, { width: '42%' }]} />
        </View>
        <View style={styles.timeLabels}>
          <Text style={styles.timeText}>{formatTime(playerState.currentTimeSeconds)}</Text>
          <Text style={styles.timeText}>{formatTime(track.durationSeconds)}</Text>
        </View>
      </View>

      <View style={styles.controlsRow}>
        <TouchableOpacity onPress={() => audioPlayer.toggleShuffle()}>
          <Ionicons name="shuffle" size={22} color={playerState.isShuffle ? ACCENT : '#9ca3af'} />
        </TouchableOpacity>
        <TouchableOpacity onPress={() => audioPlayer.previousTrack()}>
          <Ionicons name="play-skip-back" size={22} color="#f3f4f6" />
        </TouchableOpacity>

        <TouchableOpacity
          style={styles.playButton}
          onPress={() => audioPlayer.togglePlayPause()}
        >
          <Ionicons name={playerState.isPlaying ? 'pause' : 'play'} size={24} color="#0a1512" />
        </TouchableOpacity>

        <TouchableOpacity onPress={() => audioPlayer.nextTrack()}>
          <Ionicons name="play-skip-forward" size={22} color="#f3f4f6" />
        </TouchableOpacity>
        <TouchableOpacity onPress={() => audioPlayer.toggleRepeat()}>
          <Ionicons name="repeat" size={22} color={playerState.repeatMode !== 'off' ? ACCENT : '#9ca3af'} />
        </TouchableOpacity>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f131d',
    paddingTop: 40,
    paddingHorizontal: 24,
    justifyContent: 'space-between',
    paddingBottom: 40,
  },
  topBar: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
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
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  emptyTitle: { color: '#f3f4f6', fontSize: 17, fontWeight: '600', marginTop: 12 },
  emptyText: { color: '#9ca3af', fontSize: 13, textAlign: 'center', maxWidth: 240 },
  artworkContainer: { alignItems: 'center', marginVertical: 20 },
  artworkImage: {
    width: 240,
    height: 240,
    borderRadius: 24,
  },
  artworkPlaceholder: {
    width: 240,
    height: 240,
    borderRadius: 24,
    backgroundColor: ACCENT,
    alignItems: 'center',
    justifyContent: 'center',
    elevation: 8,
  },
  trackDetails: { alignItems: 'center' },
  trackTitle: { fontSize: 20, fontWeight: '700', color: '#f3f4f6', textAlign: 'center' },
  trackSubtitle: { fontSize: 14, color: '#9ca3af', marginTop: 4, textAlign: 'center' },
  qualityBadge: {
    marginTop: 8,
    flexDirection: 'row',
    alignItems: 'center',
    gap: 5,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 10,
    backgroundColor: ACCENT_TINT_20,
  },
  qualityText: { fontSize: 11, fontWeight: '600', color: ACCENT },
  progressContainer: { marginVertical: 20 },
  progressBar: {
    height: 6,
    backgroundColor: 'rgba(255, 255, 255, 0.1)',
    borderRadius: 3,
    overflow: 'hidden',
  },
  progressFill: { height: '100%', backgroundColor: ACCENT, borderRadius: 3 },
  timeLabels: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 8,
  },
  timeText: { fontSize: 11, color: '#9ca3af' },
  controlsRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-around',
    marginBottom: 20,
  },
  playButton: {
    width: 60,
    height: 60,
    borderRadius: 30,
    backgroundColor: ACCENT,
    alignItems: 'center',
    justifyContent: 'center',
    elevation: 6,
  },
});
