import * as SecureStore from 'expo-secure-store';

const SERVER_URL_KEY = 'opusflow_server_url';
const PAIRING_TOKEN_KEY = 'opusflow_pairing_token';

export interface ServerCredentials {
  serverUrl: string;
  pairingToken: string;
}

/**
 * Save server base URL and pairing token to secure storage.
 */
export async function saveServerCredentials(
  serverUrl: string,
  pairingToken: string
): Promise<void> {
  const normalizedUrl = serverUrl.trim().replace(/\/+$/, '');
  await SecureStore.setItemAsync(SERVER_URL_KEY, normalizedUrl);
  await SecureStore.setItemAsync(PAIRING_TOKEN_KEY, pairingToken.trim());
}

/**
 * Retrieve saved server URL and pairing token.
 */
export async function getServerCredentials(): Promise<ServerCredentials | null> {
  const serverUrl = await SecureStore.getItemAsync(SERVER_URL_KEY);
  const pairingToken = await SecureStore.getItemAsync(PAIRING_TOKEN_KEY);

  if (!serverUrl || !pairingToken) {
    return null;
  }

  return { serverUrl, pairingToken };
}

/**
 * Clear stored server credentials.
 */
export async function clearServerCredentials(): Promise<void> {
  await SecureStore.deleteItemAsync(SERVER_URL_KEY);
  await SecureStore.deleteItemAsync(PAIRING_TOKEN_KEY);
}

/**
 * Validate connection to server using pairing token.
 */
export async function validateServerConnection(
  serverUrl: string,
  pairingToken: string
): Promise<boolean> {
  try {
    const normalizedUrl = serverUrl.trim().replace(/\/+$/, '');
    const response = await fetch(`${normalizedUrl}/api/health`, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${pairingToken.trim()}`,
      },
    });

    return response.ok;
  } catch (error) {
    return false;
  }
}
