import { useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { StyleSheet, View } from 'react-native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';
import { ConnectScreen } from './src/screens/ConnectScreen';
import { LibraryScreen } from './src/screens/LibraryScreen';
import { PlayerScreen } from './src/screens/PlayerScreen';
import { QueueScreen } from './src/screens/QueueScreen';
import { StorageScreen } from './src/screens/StorageScreen';
import { BottomNavBar, TabType } from './src/components/BottomNavBar';
import { MiniPlayerBar } from './src/components/MiniPlayerBar';

export default function App() {
  const [currentTab, setCurrentTab] = useState<TabType>('library');
  // The queue view (backlog/027) is only ever reached from the Player
  // screen and always returns there — it isn't a bottom-tab destination,
  // so it's a full-screen overlay on top of everything else rather than
  // another TabType.
  const [showQueue, setShowQueue] = useState(false);

  const renderCurrentScreen = () => {
    switch (currentTab) {
      case 'connect':
        return <ConnectScreen onConnected={() => setCurrentTab('library')} />;
      case 'library':
        return <LibraryScreen onOpenPlayer={() => setCurrentTab('player')} />;
      case 'player':
        return <PlayerScreen onMinimize={() => setCurrentTab('library')} onShowQueue={() => setShowQueue(true)} />;
      case 'storage':
        return <StorageScreen />;
    }
  };

  if (showQueue) {
    return (
      <GestureHandlerRootView style={styles.container}>
        <QueueScreen onClose={() => setShowQueue(false)} />
        <StatusBar style="light" />
      </GestureHandlerRootView>
    );
  }

  return (
    <GestureHandlerRootView style={styles.container}>
      <View style={styles.screenArea}>{renderCurrentScreen()}</View>

      {currentTab !== 'player' && (
        <MiniPlayerBar onOpenPlayer={() => setCurrentTab('player')} />
      )}

      <BottomNavBar currentTab={currentTab} onSelectTab={setCurrentTab} />

      <StatusBar style="light" />
    </GestureHandlerRootView>
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
