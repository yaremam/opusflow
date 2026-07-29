import { Ionicons } from '@expo/vector-icons';
import { useState, useEffect } from 'react';
import { StyleSheet, Text, View, TouchableOpacity, FlatList } from 'react-native';
import {
  offlineStorage,
  StorageMetrics,
  DownloadedItem,
} from '../services/offlineStorage';
import { ACCENT } from '../theme';

export function StorageScreen() {
  const [metrics, setMetrics] = useState<StorageMetrics>(
    offlineStorage.getStorageMetrics()
  );
  const [items, setItems] = useState<DownloadedItem[]>(
    offlineStorage.getDownloadedItems()
  );

  const refreshData = () => {
    setMetrics(offlineStorage.getStorageMetrics());
    setItems(offlineStorage.getDownloadedItems());
  };

  useEffect(() => {
    refreshData();
  }, []);

  const handleClearCache = async () => {
    await offlineStorage.clearStreamCache();
    refreshData();
  };

  const handleRemoveTrack = async (id: number) => {
    await offlineStorage.removeTrack(id);
    refreshData();
  };

  const formatMB = (bytes: number) => {
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  };

  const formatGB = (bytes: number) => {
    return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
  };

  // No fixed quota (TDR 023) — the meter's "whole pie" is our own usage
  // plus whatever's still free on the device, not an arbitrary number.
  const meterTotal = metrics.totalUsedBytes + metrics.availableDiskSpaceBytes;
  const explicitPercent = (metrics.explicitDownloadBytes / meterTotal) * 100;
  const cachePercent = (metrics.lruCacheBytes / meterTotal) * 100;

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.headerTitle}>Offline Storage</Text>
      </View>

      <View style={styles.meterCard}>
        <View style={styles.meterRow}>
          <Text style={styles.meterLabel}>Storage Usage</Text>
          <Text style={styles.meterValue}>
            {formatGB(metrics.totalUsedBytes)} used · {formatGB(metrics.availableDiskSpaceBytes)} free
          </Text>
        </View>

        <View style={styles.meterTrack}>
          <View style={[styles.meterExplicit, { width: `${Math.min(explicitPercent, 100)}%` }]} />
          <View style={[styles.meterCache, { width: `${Math.min(cachePercent, 100)}%` }]} />
        </View>

        <View style={styles.legendRow}>
          <Text style={styles.legendItem}>
            <Text style={{ color: ACCENT }}>■</Text> Explicit (
            {formatMB(metrics.explicitDownloadBytes)})
          </Text>
          <Text style={styles.legendItem}>
            <Text style={{ color: '#10b981' }}>■</Text> LRU Cache (
            {formatMB(metrics.lruCacheBytes)})
          </Text>
        </View>
      </View>

      <Text style={styles.sectionTitle}>DOWNLOADED TRACKS & ALBUMS</Text>

      {items.length === 0 ? (
        <View style={styles.emptyState}>
          <Text style={styles.emptyText}>No offline tracks downloaded yet.</Text>
        </View>
      ) : (
        <FlatList
          data={items}
          keyExtractor={(item) => item.id.toString()}
          renderItem={({ item }) => (
            <View style={styles.itemRow}>
              <View style={styles.itemInfo}>
                <View style={styles.itemIcon}>
                  <Ionicons name="disc-outline" size={16} color="#9ca3af" />
                </View>
                <View style={{ flex: 1 }}>
                  <Text style={styles.itemTitle}>{item.title}</Text>
                  <Text style={styles.itemSubtitle}>
                    {item.artistName} • {formatMB(item.sizeBytes)}
                  </Text>
                </View>
              </View>

              <TouchableOpacity onPress={() => handleRemoveTrack(item.id)}>
                <Ionicons name="trash-outline" size={18} color="#9ca3af" />
              </TouchableOpacity>
            </View>
          )}
        />
      )}

      <TouchableOpacity style={styles.clearBtn} onPress={handleClearCache}>
        <Text style={styles.clearBtnText}>
          Clear Stream Cache ({formatMB(metrics.lruCacheBytes)})
        </Text>
      </TouchableOpacity>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#0f131d', paddingTop: 40, paddingHorizontal: 16 },
  header: { marginBottom: 16 },
  headerTitle: { fontSize: 24, fontWeight: '700', color: '#f3f4f6' },
  meterCard: {
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 16,
    padding: 16,
    marginBottom: 20,
  },
  meterRow: { flexDirection: 'row', justifyContent: 'space-between', marginBottom: 12 },
  meterLabel: { fontSize: 14, fontWeight: '600', color: '#f3f4f6' },
  meterValue: { fontSize: 14, fontWeight: '600', color: '#9ca3af' },
  meterTrack: {
    height: 10,
    backgroundColor: 'rgba(255, 255, 255, 0.1)',
    borderRadius: 5,
    overflow: 'hidden',
    flexDirection: 'row',
    marginBottom: 12,
  },
  meterExplicit: { backgroundColor: ACCENT, height: '100%' },
  meterCache: { backgroundColor: '#10b981', height: '100%' },
  legendRow: { flexDirection: 'row', gap: 16 },
  legendItem: { fontSize: 12, color: '#9ca3af' },
  sectionTitle: { fontSize: 12, fontWeight: '700', color: '#9ca3af', letterSpacing: 0.5, marginBottom: 12 },
  emptyState: { padding: 24, alignItems: 'center' },
  emptyText: { color: '#6b7280', fontSize: 14 },
  itemRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 12,
    marginBottom: 8,
  },
  itemInfo: { flexDirection: 'row', alignItems: 'center', flex: 1 },
  itemIcon: {
    width: 36,
    height: 36,
    borderRadius: 8,
    backgroundColor: 'rgba(255, 255, 255, 0.05)',
    alignItems: 'center',
    justifyContent: 'center',
    marginRight: 12,
  },
  itemTitle: { fontSize: 14, fontWeight: '600', color: '#f3f4f6' },
  itemSubtitle: { fontSize: 12, color: '#9ca3af', marginTop: 2 },
  clearBtn: {
    backgroundColor: 'rgba(239, 68, 68, 0.15)',
    borderWidth: 1,
    borderColor: 'rgba(239, 68, 68, 0.3)',
    borderRadius: 12,
    padding: 14,
    alignItems: 'center',
    marginVertical: 20,
  },
  clearBtnText: { color: '#f87171', fontWeight: '600', fontSize: 14 },
});
