import { Ionicons } from '@expo/vector-icons';
import { Image, StyleSheet, View } from 'react-native';
import { ACCENT } from '../theme';

interface PlaylistCoverCollageProps {
  coverUrls: string[];
  // size is a fixed pixel dimension for contexts with no square parent to
  // fill (PlaylistDetailScreen's hero). Omit it inside a flex:1 grid card
  // (PlaylistsListScreen, matching AlbumsListScreen's own responsive
  // `width: '100%', aspectRatio: 1` cover) and this fills its parent
  // instead of fighting a hardcoded size against a variable card width.
  size?: number;
  style?: object;
}

// PlaylistCoverCollage renders a playlist's cover as a 2x2 collage of its
// first up to 4 tracks' album art (AC-7) — the same derived-not-owned
// cover web's PlaylistCoverTile renders.
export function PlaylistCoverCollage({ coverUrls, size, style }: PlaylistCoverCollageProps) {
  const dims = size ? { width: size, height: size } : { width: '100%' as const, aspectRatio: 1 };
  const iconSize = size ? size * 0.4 : 30;
  const radius = size ? size * 0.14 : 12;

  if (coverUrls.length === 0) {
    return (
      <View style={[styles.placeholder, dims, { borderRadius: radius }, style]}>
        <Ionicons name="musical-notes-outline" size={iconSize} color="#0a1512" />
      </View>
    );
  }

  const cellDims = size ? { width: size / 2, height: size / 2 } : { width: '50%' as const, height: '50%' as const };
  return (
    <View style={[styles.collage, dims, { borderRadius: radius }, style]}>
      {Array.from({ length: 4 }).map((_, i) =>
        coverUrls[i] ? (
          <Image key={i} source={{ uri: coverUrls[i] }} style={cellDims} />
        ) : (
          <View key={i} style={[styles.cell, cellDims]} />
        ),
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  collage: { flexDirection: 'row', flexWrap: 'wrap', overflow: 'hidden' },
  cell: { backgroundColor: '#1a2030' },
  placeholder: { backgroundColor: ACCENT, alignItems: 'center', justifyContent: 'center' },
});
