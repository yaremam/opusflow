import { describe, it, expect, beforeEach, vi } from 'vitest';
import { fetchAlbumTracks, fetchCatalogAlbums, fetchCatalogTracks } from '../api';
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

  it('should fetch albums from the real /api/library/albums route, unwrap the page, and attach coverUrl', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockPage = {
      items: [
        { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026, coverThumbUrl: '', coverUrl: '/artwork/abc/full.jpg' },
      ],
      page: 1,
      pageSize: 100,
      totalCount: 1,
    };

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockPage,
      } as Response)
    );

    const albums = await fetchCatalogAlbums('Midnight');
    expect(albums).toEqual([
      {
        id: 1,
        title: 'Midnight Sun',
        artistName: 'Solaris',
        year: 2026,
        coverUrl: 'http://localhost:8080/artwork/abc/full.jpg',
      },
    ]);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/albums?pageSize=100&q=Midnight',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  it('falls back to the thumbnail when an album has no full cover yet', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockPage = {
      items: [{ id: 2, title: 'Neon Pulse', artistName: 'SynthWave', year: 2026, coverThumbUrl: '/artwork/thumb.jpg', coverUrl: '' }],
      page: 1,
      pageSize: 100,
      totalCount: 1,
    };

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockPage } as Response));

    const albums = await fetchCatalogAlbums();
    expect(albums[0].coverUrl).toBe('http://localhost:8080/artwork/thumb.jpg');
  });

  it('should fetch tracks from the real /api/library/songs route, unwrap the page, and generate stream/artwork URLs', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockPage = {
      items: [
        {
          id: 101,
          title: 'Cosmic Voyager',
          artistName: 'Solaris',
          albumId: 1,
          albumTitle: 'Midnight Sun',
          albumCoverThumbUrl: '/artwork/abc/thumb.jpg',
          durationSeconds: 255,
        },
      ],
      page: 1,
      pageSize: 100,
      totalCount: 1,
    };

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockPage,
      } as Response)
    );

    const tracks = await fetchCatalogTracks();
    expect(tracks).toHaveLength(1);
    expect(tracks[0].streamUrl).toBe('http://localhost:8080/api/library/songs/101/stream');
    expect(tracks[0].coverUrl).toBe('http://localhost:8080/artwork/abc/thumb.jpg');
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/songs?pageSize=100',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  // Regression coverage for issue #69: tapping an album card had no
  // onPress handler at all, so nothing happened. fetchAlbumTracks backs
  // the fix — a real per-album track listing, not a filter over whatever
  // page of fetchCatalogTracks happens to already be loaded.
  it('should fetch an album\'s real track listing from GET /api/library/albums/{id}', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockAlbumDetail = {
      id: 1,
      title: 'Midnight Sun',
      artistName: 'Solaris',
      tracks: [
        { id: 101, title: 'Cosmic Voyager', trackNumber: 1, durationSeconds: 255 },
        { id: 102, title: 'Digital Horizon', trackNumber: 2, durationSeconds: 210 },
      ],
    };

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockAlbumDetail,
      } as Response)
    );

    const tracks = await fetchAlbumTracks({
      id: 1,
      title: 'Midnight Sun',
      artistName: 'Solaris',
      coverUrl: 'http://localhost:8080/artwork/abc/full.jpg',
    });

    expect(tracks).toEqual([
      {
        id: 101,
        title: 'Cosmic Voyager',
        artistName: 'Solaris',
        albumTitle: 'Midnight Sun',
        albumId: 1,
        durationSeconds: 255,
        streamUrl: 'http://localhost:8080/api/library/songs/101/stream',
        coverUrl: 'http://localhost:8080/artwork/abc/full.jpg',
      },
      {
        id: 102,
        title: 'Digital Horizon',
        artistName: 'Solaris',
        albumTitle: 'Midnight Sun',
        albumId: 1,
        durationSeconds: 210,
        streamUrl: 'http://localhost:8080/api/library/songs/102/stream',
        coverUrl: 'http://localhost:8080/artwork/abc/full.jpg',
      },
    ]);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/library/albums/1', {
      headers: { Authorization: 'Bearer secret_token' },
    });
  });
});
