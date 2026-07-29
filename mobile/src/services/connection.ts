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

// ConnectionResult distinguishes why pairing failed (TDR 022 AC-7) — a
// wrong token and an unreachable server need different messaging on the
// Connect screen, not one generic failure.
export type ConnectionResult = 'valid' | 'unauthorized' | 'unreachable';

/**
 * Validate a server URL and pairing token together in one request. GET
 * /api/health requires auth (TDR 022) — unlike the bare /health, which
 * always succeeds and proves nothing about the token — so succeeding
 * against it is both a reachability check and a token check at once.
 * Deliberately does not fall back to the bare /health on failure: that
 * would make every wrong token look valid, since bare /health is never
 * gated.
 */
export async function validateServerConnection(
  serverUrl: string,
  pairingToken: string
): Promise<ConnectionResult> {
  const normalizedUrl = serverUrl.trim().replace(/\/+$/, '');

  let response: Response;
  try {
    response = await fetch(`${normalizedUrl}/api/health`, {
      method: 'GET',
      headers: { Authorization: `Bearer ${pairingToken.trim()}` },
    });
  } catch {
    return 'unreachable';
  }

  if (response.status === 401) return 'unauthorized';
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
