import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fetchCatalogAlbums, fetchCatalogTracks } from '../api';
import * as connection from '../connection';

vi.mock('../connection', () => ({
  getServerCredentials: vi.fn(),
}));

describe('Catalog API Client (AC-2)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should throw error when server credentials are missing', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue(null);
    await expect(fetchCatalogAlbums()).rejects.toThrow('No server credentials saved.');
  });

  it('should fetch albums with authorization header and attach coverUrl', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockAlbums = [
      { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026 },
    ];

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockAlbums,
      } as Response)
    );

    const albums = await fetchCatalogAlbums('Midnight');
    expect(albums).toEqual([
      {
        id: 1,
        title: 'Midnight Sun',
        artistName: 'Solaris',
        year: 2026,
        coverUrl: 'http://localhost:8080/api/catalog/albums/1/art',
      },
    ]);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/catalog/albums?q=Midnight',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  it('should fetch tracks and generate stream and artwork URLs', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockRawTracks = [
      {
        id: 101,
        title: 'Cosmic Voyager',
        artistName: 'Solaris',
        albumTitle: 'Midnight Sun',
        albumId: 1,
        durationSeconds: 255,
      },
    ];

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockRawTracks,
      } as Response)
    );

    const tracks = await fetchCatalogTracks();
    expect(tracks).toHaveLength(1);
    expect(tracks[0].streamUrl).toBe('http://localhost:8080/api/stream/101');
    expect(tracks[0].coverUrl).toBe('http://localhost:8080/api/catalog/albums/1/art');
  });
});
