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

// TDR 022 AC-7: an unreachable server and a rejected token have to
// surface distinctly — GET /api/health now requires auth for real, so
// succeeding against it proves both reachability and a valid token in
// one request. These mocks model the real backend's actual contract: the
// bare /health (no /api prefix) always succeeds and carries no auth
// signal at all, unlike the old implementation which fell back to it on
// any /api/health failure — making every wrong token look valid.
describe('validateServerConnection (AC-7)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns "valid" when /api/health accepts the token', async () => {
    const mockFetch = vi.fn(async (url: string, init?: RequestInit) => {
      expect(url).toBe('http://192.168.1.100:8080/api/health');
      const authHeader = (init?.headers as Record<string, string>)?.['Authorization'];
      if (authHeader === 'Bearer test_token_123') {
        return { ok: true, status: 200 } as Response;
      }
      return { ok: false, status: 401 } as Response;
    });
    vi.stubGlobal('fetch', mockFetch);

    const result = await validateServerConnection('http://192.168.1.100:8080', 'test_token_123');
    expect(result).toBe('valid');
  });

  it('returns "unauthorized" (not "valid") for a wrong token — no falling back to the always-open bare /health', async () => {
    const mockFetch = vi.fn(async (url: string) => {
      if (url === 'http://192.168.1.100:8080/api/health') {
        return { ok: false, status: 401 } as Response;
      }
      // The bare /health endpoint always succeeds on a real server
      // (TDR 022 — never gated) — if validateServerConnection still fell
      // back to it, this test would incorrectly see "valid" for a wrong
      // token.
      return { ok: true, status: 200 } as Response;
    });
    vi.stubGlobal('fetch', mockFetch);

    const result = await validateServerConnection('http://192.168.1.100:8080', 'wrong-token');
    expect(result).toBe('unauthorized');
  });

  it('returns "unreachable" when the server can\'t be reached at all', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      })
    );

    const result = await validateServerConnection('http://nope.invalid', 'any-token');
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
