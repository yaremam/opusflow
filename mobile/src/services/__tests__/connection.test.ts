import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  saveServerCredentials,
  getServerCredentials,
  clearServerCredentials,
  validateServerConnection,
  parseQrPayload,
} from '../connection';

// Mock expo-secure-store
const store: Record<string, string> = {};

vi.mock('expo-secure-store', () => ({
  setItemAsync: vi.fn(async (key: string, value: string) => {
    store[key] = value;
  }),
  getItemAsync: vi.fn(async (key: string) => {
    return store[key] || null;
  }),
  deleteItemAsync: vi.fn(async (key: string) => {
    delete store[key];
  }),
}));

describe('Connection & Token Management (AC-1)', () => {
  beforeEach(() => {
    for (const key in store) {
      delete store[key];
    }
    vi.clearAllMocks();
  });

  it('should save server URL and pairing token to secure store', async () => {
    await saveServerCredentials('http://192.168.1.100:8080', 'test_token_123');

    const credentials = await getServerCredentials();
    expect(credentials).toEqual({
      serverUrl: 'http://192.168.1.100:8080',
      pairingToken: 'test_token_123',
    });
  });

  it('should return null when credentials do not exist', async () => {
    const credentials = await getServerCredentials();
    expect(credentials).toBeNull();
  });

  it('should clear credentials from secure store', async () => {
    await saveServerCredentials('http://192.168.1.100:8080', 'test_token_123');
    await clearServerCredentials();

    const credentials = await getServerCredentials();
    expect(credentials).toBeNull();
  });
});

// TDR 024: nothing is gated anymore, so pairing checks reachability only,
// against the always-open bare /health — not /api/health, and no token is
// sent or checked at all.
describe('validateServerConnection (TDR 024)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns "valid" when the server responds to /health', async () => {
    const mockFetch = vi.fn(async (url: string) => {
      expect(url).toBe('http://192.168.1.100:8080/health');
      return { ok: true, status: 200 } as Response;
    });
    vi.stubGlobal('fetch', mockFetch);

    const result = await validateServerConnection('http://192.168.1.100:8080');
    expect(result).toBe('valid');
  });

  it('returns "unreachable" when the server can\'t be reached at all', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      })
    );

    const result = await validateServerConnection('http://nope.invalid');
    expect(result).toBe('unreachable');
  });
});

describe('parseQrPayload (AC-6)', () => {
  it('parses the {serverUrl, token} JSON web encodes into the QR code', () => {
    const data = JSON.stringify({ serverUrl: 'http://192.168.1.100:8080', token: 'opusflow_pt_abc123' });
    expect(parseQrPayload(data)).toEqual({ serverUrl: 'http://192.168.1.100:8080', token: 'opusflow_pt_abc123' });
  });

  it('rejects a QR code that decoded to something else entirely', () => {
    expect(parseQrPayload('not json at all')).toBeNull();
  });

  it('rejects valid JSON missing the fields it needs', () => {
    expect(parseQrPayload(JSON.stringify({ serverUrl: 'http://x' }))).toBeNull();
    expect(parseQrPayload(JSON.stringify({ token: 'abc' }))).toBeNull();
    expect(parseQrPayload(JSON.stringify({ serverUrl: '', token: '' }))).toBeNull();
  });
});
