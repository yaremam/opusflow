import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  saveServerCredentials,
  getServerCredentials,
  clearServerCredentials,
  validateServerConnection,
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

  it('should validate server connection by checking health endpoint with pairing token header', async () => {
    const mockFetch = vi.fn(async (url: string, init?: RequestInit) => {
      if (url === 'http://192.168.1.100:8080/api/health') {
        const authHeader = (init?.headers as Record<string, string>)?.[
          'Authorization'
        ];
        if (authHeader === 'Bearer test_token_123') {
          return { ok: true, status: 200, json: async () => ({ status: 'ok' }) } as Response;
        }
      }
      return { ok: false, status: 401, json: async () => ({}) } as Response;
    });

    vi.stubGlobal('fetch', mockFetch);

    const isValid = await validateServerConnection('http://192.168.1.100:8080', 'test_token_123');
    expect(isValid).toBe(true);

    const isInvalid = await validateServerConnection('http://192.168.1.100:8080', 'wrong_token');
    expect(isInvalid).toBe(false);
  });
});
