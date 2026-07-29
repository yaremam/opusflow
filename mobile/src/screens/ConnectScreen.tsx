import { Ionicons } from '@expo/vector-icons';
import { CameraView, useCameraPermissions } from 'expo-camera';
import { useEffect, useState } from 'react';
import { Image, StyleSheet, Text, View, TextInput, TouchableOpacity, ActivityIndicator } from 'react-native';
import {
  saveServerCredentials,
  getServerCredentials,
  validateServerConnection,
  parseQrPayload,
  ServerCredentials,
} from '../services/connection';
import { ACCENT } from '../theme';

interface ConnectScreenProps {
  onConnected?: (credentials: ServerCredentials) => void;
}

// ConnectScreen defaults to scanning the QR code Web Settings shows after
// generating a pairing token (TDR 022 AC-6) — "Enter manually" is the
// fallback for a camera-less emulator, a denied permission, or someone
// who'd rather type it, covering exactly the same two fields the form
// already had.
export function ConnectScreen({ onConnected }: ConnectScreenProps) {
  const [mode, setMode] = useState<'scan' | 'manual'>('scan');
  const [serverUrl, setServerUrl] = useState('');
  const [pairingToken, setPairingToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [isSuccess, setIsSuccess] = useState(false);
  const [scanLocked, setScanLocked] = useState(false);

  const [permission, requestPermission] = useCameraPermissions();

  useEffect(() => {
    (async () => {
      const saved = await getServerCredentials();
      if (saved) {
        setServerUrl(saved.serverUrl);
        setPairingToken(saved.pairingToken);
      }
    })();
  }, []);

  async function attemptConnect(url: string, token: string) {
    setLoading(true);
    setStatusMessage(null);

    const result = await validateServerConnection(url, token);

    if (result === 'valid') {
      await saveServerCredentials(url, token);
      setIsSuccess(true);
      setStatusMessage('Connected & paired successfully!');
      onConnected?.({ serverUrl: url, pairingToken: token });
    } else if (result === 'unauthorized') {
      setIsSuccess(false);
      setStatusMessage("That pairing token wasn't accepted. Generate a new one from Web Settings.");
    } else {
      setIsSuccess(false);
      setStatusMessage("Couldn't reach that server. Check the address and your network.");
    }

    setLoading(false);
  }

  function handleQrScanned(data: string) {
    if (scanLocked) return;
    const payload = parseQrPayload(data);
    if (!payload) {
      setIsSuccess(false);
      setStatusMessage("That QR code isn't an OpusFlow pairing code.");
      return;
    }
    setScanLocked(true);
    setServerUrl(payload.serverUrl);
    setPairingToken(payload.token);
    attemptConnect(payload.serverUrl, payload.token).finally(() => setScanLocked(false));
  }

  function handleManualConnect() {
    if (!serverUrl.trim() || !pairingToken.trim()) {
      setIsSuccess(false);
      setStatusMessage('Please provide both Server URL and Pairing Token.');
      return;
    }
    attemptConnect(serverUrl.trim(), pairingToken.trim());
  }

  return (
    <View style={styles.container}>
      <View style={styles.headerContainer}>
        <Image source={require('../../assets/icon.png')} style={styles.logoBadge} />
        <Text style={styles.title}>OpusFlow Mobile</Text>
        <Text style={styles.subtitle}>Connect to your self-hosted music server</Text>
      </View>

      {mode === 'scan' ? (
        <>
          <Text style={styles.scanTitle}>Scan to pair</Text>
          <Text style={styles.scanSub}>Point your camera at the QR code in Web Settings</Text>

          {permission?.granted ? (
            <View style={styles.scanFrame}>
              <CameraView
                style={StyleSheet.absoluteFill}
                facing="back"
                barcodeScannerSettings={{ barcodeTypes: ['qr'] }}
                onBarcodeScanned={(result) => handleQrScanned(result.data)}
              />
              {loading && (
                <View style={styles.scanOverlay}>
                  <ActivityIndicator color="#ffffff" />
                </View>
              )}
            </View>
          ) : (
            <View style={[styles.scanFrame, styles.scanFramePlaceholder]}>
              <Ionicons name="camera-outline" size={30} color="#6b7280" />
              <Text style={styles.scanPermissionText}>
                {permission?.canAskAgain === false
                  ? 'Camera access was denied — enable it in system settings, or enter manually below.'
                  : 'OpusFlow needs camera access to scan the pairing QR code.'}
              </Text>
              {permission?.canAskAgain !== false && (
                <TouchableOpacity style={styles.enableCameraButton} onPress={requestPermission}>
                  <Text style={styles.enableCameraButtonText}>Enable Camera</Text>
                </TouchableOpacity>
              )}
            </View>
          )}

          <View style={styles.divider}>
            <View style={styles.dividerLine} />
            <Text style={styles.dividerText}>OR</Text>
            <View style={styles.dividerLine} />
          </View>

          <TouchableOpacity style={styles.secondaryButton} onPress={() => setMode('manual')}>
            <Text style={styles.secondaryButtonText}>Enter manually</Text>
          </TouchableOpacity>
        </>
      ) : (
        <>
          <TouchableOpacity style={styles.backRow} onPress={() => setMode('scan')}>
            <Ionicons name="chevron-back" size={16} color="#9ca3af" />
            <Text style={styles.backText}>Scan QR instead</Text>
          </TouchableOpacity>

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

          <TouchableOpacity style={styles.button} onPress={handleManualConnect} disabled={loading}>
            {loading ? <ActivityIndicator color="#0a1512" /> : <Text style={styles.buttonText}>Connect &amp; Sync Library</Text>}
          </TouchableOpacity>
        </>
      )}

      {statusMessage && (
        <View style={[styles.statusCard, isSuccess ? styles.statusSuccess : styles.statusError]}>
          <View style={styles.statusRow}>
            <Ionicons name={isSuccess ? 'checkmark-circle' : 'close-circle'} size={16} color={isSuccess ? '#10b981' : '#ef4444'} />
            <Text style={styles.statusText}>{statusMessage}</Text>
          </View>
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
    marginBottom: 24,
  },
  logoBadge: {
    width: 64,
    height: 64,
    borderRadius: 20,
    marginBottom: 12,
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
    textAlign: 'center',
  },
  scanTitle: {
    fontSize: 17,
    fontWeight: '700',
    color: '#f3f4f6',
    textAlign: 'center',
    marginTop: 8,
  },
  scanSub: {
    fontSize: 12,
    color: '#9ca3af',
    textAlign: 'center',
    marginTop: 4,
    marginBottom: 18,
  },
  scanFrame: {
    aspectRatio: 1,
    borderRadius: 18,
    overflow: 'hidden',
    backgroundColor: '#141824',
  },
  scanFramePlaceholder: {
    borderWidth: 2,
    borderColor: 'rgba(255, 255, 255, 0.08)',
    borderStyle: 'dashed',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
    gap: 10,
  },
  scanPermissionText: {
    color: '#9ca3af',
    fontSize: 12,
    textAlign: 'center',
  },
  enableCameraButton: {
    backgroundColor: ACCENT,
    borderRadius: 10,
    paddingVertical: 10,
    paddingHorizontal: 18,
    marginTop: 4,
  },
  enableCameraButtonText: {
    color: '#0a1512',
    fontSize: 13,
    fontWeight: '700',
  },
  scanOverlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: 'rgba(15, 19, 29, 0.55)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  divider: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    marginVertical: 18,
  },
  dividerLine: { flex: 1, height: 1, backgroundColor: 'rgba(255, 255, 255, 0.08)' },
  dividerText: { fontSize: 11, color: '#6b7280' },
  secondaryButton: {
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.12)',
    borderRadius: 14,
    padding: 14,
    alignItems: 'center',
  },
  secondaryButtonText: { color: '#f3f4f6', fontSize: 14, fontWeight: '600' },
  backRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 4,
    marginBottom: 16,
  },
  backText: { color: '#9ca3af', fontSize: 13, fontWeight: '600' },
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
    backgroundColor: ACCENT,
    borderRadius: 14,
    padding: 16,
    alignItems: 'center',
    marginTop: 8,
  },
  buttonText: {
    color: '#0a1512',
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
  statusRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  statusText: {
    color: '#f3f4f6',
    fontSize: 14,
    flexShrink: 1,
  },
});
