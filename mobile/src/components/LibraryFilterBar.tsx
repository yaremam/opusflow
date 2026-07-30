import { Ionicons } from '@expo/vector-icons';
import { ScrollView, StyleSheet, Text, TextInput, TouchableOpacity, View } from 'react-native';
import { ListParams } from '../services/api';
import { ACCENT, ACCENT_TINT_20 } from '../theme';

interface LibraryFilterBarProps {
  filters: ListParams;
  onChange: (next: ListParams) => void;
  searchPlaceholder: string;
}

// LibraryFilterBar is the search + sort + genre + year row shared by
// Artists/Albums/Songs (backlog/026 AC-2) — full parity with web's own
// filter set on each of its three list pages.
export function LibraryFilterBar({ filters, onChange, searchPlaceholder }: LibraryFilterBarProps) {
  return (
    <View>
      <View style={styles.search}>
        <Ionicons name="search-outline" size={16} color="#6b7280" style={styles.searchIcon} />
        <TextInput
          style={styles.searchInput}
          placeholder={searchPlaceholder}
          placeholderTextColor="#6b7280"
          value={filters.q ?? ''}
          onChangeText={(q) => onChange({ ...filters, q })}
        />
      </View>

      <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.filterRow}>
        <TouchableOpacity
          style={[styles.chip, filters.sort === 'name' && styles.chipOn]}
          onPress={() => onChange({ ...filters, sort: filters.sort === 'name' ? 'recent' : 'name' })}
        >
          <Text style={[styles.chipText, filters.sort === 'name' && styles.chipTextOn]}>
            Sort: {filters.sort === 'name' ? 'Name' : 'Recent'}
          </Text>
        </TouchableOpacity>

        <View style={[styles.chip, styles.chipInput, filters.genre && styles.chipOn]}>
          <TextInput
            style={[styles.chipTextInput, filters.genre && styles.chipTextOn]}
            placeholder="Genre"
            placeholderTextColor="#6b7280"
            value={filters.genre ?? ''}
            onChangeText={(genre) => onChange({ ...filters, genre: genre || undefined })}
          />
        </View>

        <View style={[styles.chip, styles.chipInput, !!filters.year && styles.chipOn]}>
          <TextInput
            style={[styles.chipTextInput, !!filters.year && styles.chipTextOn]}
            placeholder="Year"
            placeholderTextColor="#6b7280"
            keyboardType="number-pad"
            value={filters.year ? String(filters.year) : ''}
            onChangeText={(year) => onChange({ ...filters, year: year ? parseInt(year, 10) || undefined : undefined })}
          />
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
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
  filterRow: { flexDirection: 'row', marginTop: 8, marginBottom: 4 },
  chip: {
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 999,
    paddingHorizontal: 12,
    paddingVertical: 6,
    marginRight: 8,
  },
  chipOn: { borderColor: ACCENT, backgroundColor: ACCENT_TINT_20 },
  chipText: { fontSize: 11, fontWeight: '600', color: '#9ca3af' },
  chipTextOn: { color: ACCENT },
  chipInput: { minWidth: 72 },
  chipTextInput: { fontSize: 11, fontWeight: '600', color: '#9ca3af', padding: 0, minWidth: 48 },
});
