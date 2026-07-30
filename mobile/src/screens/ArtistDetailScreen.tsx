import { Ionicons } from '@expo/vector-icons';
import { useEffect, useState } from 'react';
import { FlatList, Image, StyleSheet, Text, TouchableOpacity, View } from 'react-native';
import { fetchArtistDetail, Album, ArtistDetail } from '../services/api';
import { ACCENT } from '../theme';

interface ArtistDetailScreenProps {
  artistId: number;
  onBack: () => void;
  onOpenAlbum: (album: Album) => void;
}

// ArtistDetailScreen matches web's ArtistDetailPage content (backlog/026
// AC-3, confirmed in grilling): bio/facts when available, plus a grid of
// the artist's albums.
export function ArtistDetailScreen({ artistId, onBack, onOpenAlbum }: ArtistDetailScreenProps) {
  const [detail, setDetail] = useState<ArtistDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchArtistDetail(artistId)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Could not load this artist.');
      });
    return () => {
      cancelled = true;
    };
  }, [artistId]);

  const facts = detail
    ? [detail.formedYear ? `Formed ${detail.formedYear}` : null, detail.country, detail.genres.join(', ') || null]
        .filter(Boolean)
        .join(' · ')
    : '';

  return (
    <View style={styles.container}>
      <View style={styles.topBar}>
        <TouchableOpacity style={styles.iconBtn} onPress={onBack}>
          <Ionicons name="chevron-back" size={18} color="#f3f4f6" />
        </TouchableOpacity>
        <Text style={styles.topTitle}>ARTIST</Text>
        <View style={styles.iconBtn} />
      </View>

      {error ? (
        <View style={styles.emptyState}>
          <Ionicons name="cloud-offline-outline" size={28} color="#6b7280" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : !detail ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyText}>Loading…</Text>
        </View>
      ) : (
        <FlatList
          data={detail.albums}
          keyExtractor={(item) => item.id.toString()}
          numColumns={2}
          columnWrapperStyle={styles.albumRow}
          contentContainerStyle={styles.listContent}
          ListHeaderComponent={
            <>
              <View style={styles.hero}>
                {detail.photoUrl ? (
                  <Image source={{ uri: detail.photoUrl }} style={styles.heroAvatar} />
                ) : (
                  <View style={[styles.heroAvatar, styles.heroAvatarPlaceholder]}>
                    <Text style={styles.heroInitial}>{detail.name.charAt(0).toUpperCase()}</Text>
                  </View>
                )}
                <Text style={styles.heroName}>{detail.name}</Text>
                {facts ? <Text style={styles.heroFacts}>{facts}</Text> : null}
              </View>
              {detail.bio ? <Text style={styles.bio}>{detail.bio}</Text> : null}
              <Text style={styles.sectionLabel}>Albums</Text>
            </>
          }
          ListEmptyComponent={<Text style={styles.emptyText}>No albums yet.</Text>}
          renderItem={({ item }) => (
            <TouchableOpacity style={styles.albumCard} onPress={() => onOpenAlbum(item)}>
              {item.coverUrl ? (
                <Image source={{ uri: item.coverUrl }} style={styles.albumCover} />
              ) : (
                <View style={[styles.albumCover, styles.albumCoverPlaceholder]}>
                  <Ionicons name="disc-outline" size={26} color="#ffffff" />
                </View>
              )}
              <Text style={styles.albumTitle} numberOfLines={1}>{item.title}</Text>
              {item.year ? <Text style={styles.albumYear}>{item.year}</Text> : null}
            </TouchableOpacity>
          )}
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
  emptyState: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: 6 },
  emptyText: { color: '#6b7280', fontSize: 13 },
  errorText: { color: '#9ca3af', fontSize: 13, textAlign: 'center' },
  listContent: { paddingHorizontal: 16, paddingBottom: 100 },
  hero: { alignItems: 'center', marginBottom: 12 },
  heroAvatar: { width: 76, height: 76, borderRadius: 38, marginBottom: 10 },
  heroAvatarPlaceholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
  heroInitial: { fontSize: 26, fontWeight: '700', color: '#06231f' },
  heroName: { fontSize: 18, fontWeight: '700', color: '#f3f4f6' },
  heroFacts: { fontSize: 11, color: '#9ca3af', marginTop: 3, textAlign: 'center' },
  bio: { fontSize: 12, lineHeight: 18, color: '#9ca3af', marginBottom: 16, textAlign: 'center' },
  sectionLabel: {
    fontSize: 11,
    fontWeight: '700',
    letterSpacing: 0.5,
    textTransform: 'uppercase',
    color: '#9ca3af',
    marginBottom: 10,
  },
  albumRow: { gap: 12 },
  albumCard: { flex: 1, marginBottom: 16 },
  albumCover: { width: '100%', aspectRatio: 1, borderRadius: 12, marginBottom: 6 },
  albumCoverPlaceholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
  albumTitle: { fontSize: 12, fontWeight: '600', color: '#f3f4f6' },
  albumYear: { fontSize: 10.5, color: '#9ca3af', marginTop: 1 },
});
