import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TouchableOpacity, Image } from 'react-native';
import { audioPlayer, AudioPlayerState } from '../services/audioPlayer';

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

  const track = playerState.currentTrack || {
    title: 'Cosmic Voyager',
    artistName: 'Solaris',
    albumTitle: 'Midnight Sun (2026)',
    durationSeconds: 255,
    coverUrl: undefined,
    localCoverUrl: undefined,
  };

  const activeArtworkUri = track.localCoverUrl || track.coverUrl;

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onMinimize}>
          <Text style={{ fontSize: 18, color: '#f3f4f6' }}>∨</Text>
        </TouchableOpacity>
        <Text style={styles.topTitle}>NOW PLAYING</Text>
        <TouchableOpacity style={styles.iconBtn}>
          <Text style={{ fontSize: 18, color: '#f3f4f6' }}>📄</Text>
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
            <Text style={{ fontSize: 72 }}>🌌</Text>
          </View>
        )}
      </View>

      <View style={styles.trackDetails}>
        <Text style={styles.trackTitle}>{track.title}</Text>
        <Text style={styles.trackSubtitle}>
          {track.artistName} — {track.albumTitle}
        </Text>
        <View style={styles.qualityBadge}>
          <Text style={styles.qualityText}>
            {track.localCoverUrl ? '💾 Offline Cached Artwork' : '📶 FLAC 24bit / 96kHz'}
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
          <Text style={[styles.controlIcon, playerState.isShuffle && styles.activeControl]}>
            🔀
          </Text>
        </TouchableOpacity>
        <TouchableOpacity onPress={() => audioPlayer.previousTrack()}>
          <Text style={styles.controlIcon}>⏮️</Text>
        </TouchableOpacity>

        <TouchableOpacity
          style={styles.playButton}
          onPress={() => audioPlayer.togglePlayPause()}
        >
          <Text style={{ fontSize: 24, color: '#ffffff' }}>
            {playerState.isPlaying ? '⏸️' : '▶️'}
          </Text>
        </TouchableOpacity>

        <TouchableOpacity onPress={() => audioPlayer.nextTrack()}>
          <Text style={styles.controlIcon}>⏭️</Text>
        </TouchableOpacity>
        <TouchableOpacity onPress={() => audioPlayer.toggleRepeat()}>
          <Text
            style={[
              styles.controlIcon,
              playerState.repeatMode !== 'off' && styles.activeControl,
            ]}
          >
            🔁
          </Text>
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
    backgroundColor: '#6366f1',
    alignItems: 'center',
    justifyContent: 'center',
    elevation: 8,
  },
  trackDetails: { alignItems: 'center' },
  trackTitle: { fontSize: 20, fontWeight: '700', color: '#f3f4f6', textAlign: 'center' },
  trackSubtitle: { fontSize: 14, color: '#9ca3af', marginTop: 4, textAlign: 'center' },
  qualityBadge: {
    marginTop: 8,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 10,
    backgroundColor: 'rgba(99, 102, 241, 0.2)',
  },
  qualityText: { fontSize: 11, fontWeight: '600', color: '#818cf8' },
  progressContainer: { marginVertical: 20 },
  progressBar: {
    height: 6,
    backgroundColor: 'rgba(255, 255, 255, 0.1)',
    borderRadius: 3,
    overflow: 'hidden',
  },
  progressFill: { height: '100%', backgroundColor: '#6366f1', borderRadius: 3 },
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
  controlIcon: { fontSize: 22, opacity: 0.8 },
  activeControl: { opacity: 1 },
  playButton: {
    width: 60,
    height: 60,
    borderRadius: 30,
    backgroundColor: '#6366f1',
    alignItems: 'center',
    justifyContent: 'center',
    elevation: 6,
  },
});
