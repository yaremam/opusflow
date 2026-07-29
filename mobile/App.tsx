import { useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { StyleSheet, View } from 'react-native';
import { ConnectScreen } from './src/screens/ConnectScreen';
import { LibraryScreen } from './src/screens/LibraryScreen';
import { PlayerScreen } from './src/screens/PlayerScreen';
import { StorageScreen } from './src/screens/StorageScreen';
import { BottomNavBar, TabType } from './src/components/BottomNavBar';
import { MiniPlayerBar } from './src/components/MiniPlayerBar';

export default function App() {
  const [currentTab, setCurrentTab] = useState<TabType>('library');

  const renderCurrentScreen = () => {
    switch (currentTab) {
      case 'connect':
        return <ConnectScreen onConnected={() => setCurrentTab('library')} />;
      case 'library':
        return <LibraryScreen onOpenPlayer={() => setCurrentTab('player')} />;
      case 'player':
        return <PlayerScreen onMinimize={() => setCurrentTab('library')} />;
      case 'storage':
        return <StorageScreen />;
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.screenArea}>{renderCurrentScreen()}</View>

      {currentTab !== 'player' && (
        <MiniPlayerBar onOpenPlayer={() => setCurrentTab('player')} />
      )}

      <BottomNavBar currentTab={currentTab} onSelectTab={setCurrentTab} />

      <StatusBar style="light" />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f131d',
  },
  screenArea: {
    flex: 1,
  },
});
