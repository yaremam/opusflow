import React, { useState, useEffect } from 'react';
import {
  StyleSheet,
  Text,
  View,
  TextInput,
  TouchableOpacity,
  ActivityIndicator,
} from 'react-native';
import {
  saveServerCredentials,
  getServerCredentials,
  validateServerConnection,
  ServerCredentials,
} from '../services/connection';

interface ConnectScreenProps {
  onConnected?: (credentials: ServerCredentials) => void;
}

export function ConnectScreen({ onConnected }: ConnectScreenProps) {
  const [serverUrl, setServerUrl] = useState('');
  const [pairingToken, setPairingToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);

  useEffect(() => {
    (async () => {
      const saved = await getServerCredentials();
      if (saved) {
        setServerUrl(saved.serverUrl);
        setPairingToken(saved.pairingToken);
      }
    })();
  }, []);

  const handleConnect = async () => {
    if (!serverUrl.trim() || !pairingToken.trim()) {
      setStatusMessage('Please provide both Server URL and Pairing Token.');
      setIsSuccess(false);
      return;
    }

    setLoading(true);
    setStatusMessage(null);

    const valid = await validateServerConnection(serverUrl, pairingToken);

    if (valid) {
      await saveServerCredentials(serverUrl, pairingToken);
      setIsSuccess(true);
      setStatusMessage('Connected & paired successfully!');
      onConnected?.({ serverUrl, pairingToken });
    } else {
      setIsSuccess(false);
      setStatusMessage('Connection failed. Check server URL and pairing token.');
    }

    setLoading(false);
  };

  return (
    <View style={styles.container}>
      <View style={styles.headerContainer}>
        <View style={styles.logoBadge}>
          <Text style={styles.logoText}>🎵</Text>
        </View>
        <Text style={styles.title}>OpusFlow Mobile</Text>
        <Text style={styles.subtitle}>Connect to your self-hosted music server</Text>
      </View>

      <View style={styles.formGroup}>
        <Text style={styles.label}>Server Address</Text>
        <TextInput
          style={styles.input}
          placeholder="http://192.168.1.100:8080"
          placeholderTextColor="#6b7280"
          value={serverUrl}
          onChangeText={setServerUrl}
          autoCapitalize="none"
          autoCorrect={false}
        />
      </View>

      <View style={styles.formGroup}>
        <Text style={styles.label}>API Pairing Token</Text>
        <TextInput
          style={styles.input}
          placeholder="Paste token from Web Settings"
          placeholderTextColor="#6b7280"
          value={pairingToken}
          onChangeText={setPairingToken}
          secureTextEntry
          autoCapitalize="none"
          autoCorrect={false}
        />
      </View>

      <TouchableOpacity
        style={styles.button}
        onPress={handleConnect}
        disabled={loading}
      >
        {loading ? (
          <ActivityIndicator color="#ffffff" />
        ) : (
          <Text style={styles.buttonText}>Connect & Sync Library</Text>
        )}
      </TouchableOpacity>

      {statusMessage && (
        <View
          style={[
            styles.statusCard,
            isSuccess ? styles.statusSuccess : styles.statusError,
          ]}
        >
          <Text style={styles.statusText}>
            {isSuccess ? '✅ ' : '❌ '}
            {statusMessage}
          </Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f131d',
    padding: 24,
    justifyContent: 'center',
  },
  headerContainer: {
    alignItems: 'center',
    marginBottom: 32,
  },
  logoBadge: {
    width: 64,
    height: 64,
    borderRadius: 20,
    backgroundColor: '#6366f1',
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: 12,
  },
  logoText: {
    fontSize: 32,
  },
  title: {
    fontSize: 24,
    fontWeight: '700',
    color: '#f3f4f6',
  },
  subtitle: {
    fontSize: 14,
    color: '#9ca3af',
    marginTop: 4,
  },
  formGroup: {
    marginBottom: 20,
  },
  label: {
    fontSize: 12,
    fontWeight: '600',
    color: '#9ca3af',
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginBottom: 8,
  },
  input: {
    backgroundColor: '#141824',
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderRadius: 12,
    padding: 14,
    color: '#f3f4f6',
    fontSize: 15,
  },
  button: {
    backgroundColor: '#6366f1',
    borderRadius: 14,
    padding: 16,
    alignItems: 'center',
    marginTop: 8,
  },
  buttonText: {
    color: '#ffffff',
    fontSize: 16,
    fontWeight: '600',
  },
  statusCard: {
    marginTop: 20,
    padding: 14,
    borderRadius: 12,
    borderWidth: 1,
  },
  statusSuccess: {
    backgroundColor: 'rgba(16, 185, 129, 0.1)',
    borderColor: 'rgba(16, 185, 129, 0.3)',
  },
  statusError: {
    backgroundColor: 'rgba(239, 68, 68, 0.1)',
    borderColor: 'rgba(239, 68, 68, 0.3)',
  },
  statusText: {
    color: '#f3f4f6',
    fontSize: 14,
  },
});
