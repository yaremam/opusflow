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

// ConnectionResult (TDR 024): nothing is gated anymore, so pairing has
// exactly one thing left to check — is this URL a reachable opusflow
// server at all. The token is still saved afterward purely so requests
// keep the Paired Devices list's "last used" column accurate — it's
// identity, not a credential being verified here.
export type ConnectionResult = 'valid' | 'unreachable';

/**
 * Confirm serverUrl points at a reachable opusflow server via the
 * always-open GET /health.
 */
export async function validateServerConnection(serverUrl: string): Promise<ConnectionResult> {
  const normalizedUrl = serverUrl.trim().replace(/\/+$/, '');

  let response: Response;
  try {
    response = await fetch(`${normalizedUrl}/health`, { method: 'GET' });
  } catch {
    return 'unreachable';
  }

  return response.ok ? 'valid' : 'unreachable';
}

// QrPairingPayload is what web/src/pages/SettingsPage.tsx encodes into
// the pairing QR code (JSON.stringify({ serverUrl, token })) — kept as a
// pure function so it's testable without a camera (TDR 022 AC-6).
export interface QrPairingPayload {
  serverUrl: string;
  token: string;
}

export function parseQrPayload(data: string): QrPairingPayload | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(data);
  } catch {
    return null;
  }
  if (
    typeof parsed === 'object' &&
    parsed !== null &&
    'serverUrl' in parsed &&
    'token' in parsed &&
    typeof (parsed as Record<string, unknown>).serverUrl === 'string' &&
    typeof (parsed as Record<string, unknown>).token === 'string' &&
    (parsed as Record<string, string>).serverUrl &&
    (parsed as Record<string, string>).token
  ) {
    return { serverUrl: (parsed as QrPairingPayload).serverUrl, token: (parsed as QrPairingPayload).token };
  }
  return null;
}
