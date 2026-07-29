import { StyleSheet, Text, View, TouchableOpacity } from 'react-native';

export type TabType = 'connect' | 'library' | 'player' | 'storage';

interface BottomNavBarProps {
  currentTab: TabType;
  onSelectTab: (tab: TabType) => void;
}

export function BottomNavBar({ currentTab, onSelectTab }: BottomNavBarProps) {
  const tabs: { id: TabType; label: string; icon: string }[] = [
    { id: 'connect', label: 'Connect', icon: '📶' },
    { id: 'library', label: 'Library', icon: '🎵' },
    { id: 'player', label: 'Player', icon: '🎧' },
    { id: 'storage', label: 'Offline', icon: '💾' },
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
            <Text style={[styles.tabIcon, isActive && styles.activeText]}>
              {tab.icon}
            </Text>
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
  tabIcon: { fontSize: 18, color: '#6b7280', marginBottom: 2 },
  tabLabel: { fontSize: 11, color: '#6b7280', fontWeight: '500' },
  activeText: { color: '#6366f1' },
});
