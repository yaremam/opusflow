import { Ionicons } from '@expo/vector-icons';
import { StyleSheet, Text, View, TouchableOpacity } from 'react-native';
import { ACCENT } from '../theme';

export type TabType = 'connect' | 'library' | 'player' | 'storage';

interface BottomNavBarProps {
  currentTab: TabType;
  onSelectTab: (tab: TabType) => void;
}

export function BottomNavBar({ currentTab, onSelectTab }: BottomNavBarProps) {
  const tabs: { id: TabType; label: string; icon: keyof typeof Ionicons.glyphMap }[] = [
    { id: 'connect', label: 'Connect', icon: 'wifi-outline' },
    { id: 'library', label: 'Library', icon: 'albums-outline' },
    { id: 'player', label: 'Player', icon: 'disc-outline' },
    { id: 'storage', label: 'Offline', icon: 'download-outline' },
  ];

  return (
    <View style={styles.container}>
      {tabs.map((tab) => {
        const isActive = currentTab === tab.id;
        return (
          <TouchableOpacity
            key={tab.id}
            style={styles.tabItem}
            onPress={() => onSelectTab(tab.id)}
          >
            <Ionicons name={tab.icon} size={20} color={isActive ? ACCENT : '#6b7280'} style={styles.tabIcon} />
            <Text style={[styles.tabLabel, isActive && styles.activeText]}>
              {tab.label}
            </Text>
          </TouchableOpacity>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    height: 64,
    backgroundColor: '#0a0d14',
    borderTopWidth: 1,
    borderTopColor: 'rgba(255, 255, 255, 0.08)',
    flexDirection: 'row',
    justifyContent: 'space-around',
    alignItems: 'center',
  },
  tabItem: { alignItems: 'center', justifyContent: 'center' },
  tabIcon: { marginBottom: 2 },
  tabLabel: { fontSize: 11, color: '#6b7280', fontWeight: '500' },
  activeText: { color: ACCENT },
});
