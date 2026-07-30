import { describe, it, expect, beforeEach, vi } from 'vitest';
import {
  addTrackToPlaylist,
  createPlaylist,
  deletePlaylist,
  fetchAlbumTracks,
  fetchAlbumsPage,
  fetchArtistDetail,
  fetchArtistsPage,
  fetchPlaylistDetail,
  fetchPlaylistsContainingTrack,
  fetchPlaylistsPage,
  fetchSongsPage,
  removePlaylistTrack,
  renamePlaylist,
  reorderPlaylistTracks,
} from '../api';
import * as connection from '../connection';

vi.mock('../connection', () => ({
  getServerCredentials: vi.fn(),
}));

describe('Catalog API Client (AC-2, backlog/026)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should throw error when server credentials are missing', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue(null);
    await expect(fetchAlbumsPage()).rejects.toThrow('No server credentials saved.');
  });

  it('should fetch a page of albums from the real /api/library/albums route and attach coverUrl', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockPage = {
      items: [
        { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026, coverThumbUrl: '', coverUrl: '/artwork/abc/full.jpg' },
      ],
      page: 1,
      pageSize: 30,
      totalCount: 1,
    };

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockPage,
      } as Response)
    );

    const page = await fetchAlbumsPage({ q: 'Midnight' });
    expect(page.items).toEqual([
      {
        id: 1,
        title: 'Midnight Sun',
        artistName: 'Solaris',
        year: 2026,
        coverUrl: 'http://localhost:8080/artwork/abc/full.jpg',
      },
    ]);
    expect(page.totalCount).toBe(1);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/albums?page=1&pageSize=30&q=Midnight',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  it('requests subsequent pages and every filter (sort/genre/year) for infinite scroll', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ items: [], page: 3, pageSize: 30, totalCount: 0 }),
      } as Response)
    );

    await fetchAlbumsPage({ page: 3, sort: 'name', genre: 'Ambient', year: 2020 });
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/albums?page=3&pageSize=30&sort=name&genre=Ambient&year=2020',
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
      pageSize: 30,
      totalCount: 1,
    };

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockPage } as Response));

    const page = await fetchAlbumsPage();
    expect(page.items[0].coverUrl).toBe('http://localhost:8080/artwork/thumb.jpg');
  });

  it('should fetch a page of songs from the real /api/library/songs route and generate stream/artwork URLs', async () => {
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
          format: 'flac',
          bitrateKbps: 940,
        },
      ],
      page: 1,
      pageSize: 30,
      totalCount: 1,
    };

    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => mockPage,
      } as Response)
    );

    const page = await fetchSongsPage();
    expect(page.items).toHaveLength(1);
    expect(page.items[0].streamUrl).toBe('http://localhost:8080/api/library/songs/101/stream');
    expect(page.items[0].coverUrl).toBe('http://localhost:8080/artwork/abc/thumb.jpg');
    expect(page.items[0].format).toBe('flac');
    expect(page.items[0].bitrateKbps).toBe(940);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/songs?page=1&pageSize=30',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  it('should fetch a page of artists from the real /api/library/artists route', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockPage = {
      items: [{ id: 5, name: 'Solaris', albumCount: 4, photoThumbUrl: '/artwork/solaris/thumb.jpg' }],
      page: 1,
      pageSize: 30,
      totalCount: 1,
    };

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockPage } as Response));

    const page = await fetchArtistsPage();
    expect(page.items).toEqual([
      { id: 5, name: 'Solaris', albumCount: 4, photoUrl: 'http://localhost:8080/artwork/solaris/thumb.jpg' },
    ]);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/library/artists?page=1&pageSize=30',
      { headers: { Authorization: 'Bearer secret_token' } }
    );
  });

  it('should fetch an artist\'s bio/facts and albums from GET /api/library/artists/{id}', async () => {
    vi.mocked(connection.getServerCredentials).mockResolvedValue({
      serverUrl: 'http://localhost:8080',
      pairingToken: 'secret_token',
    });

    const mockDetail = {
      id: 5,
      name: 'Solaris',
      photoUrl: '/artwork/solaris/full.jpg',
      formedYear: 2019,
      country: 'Iceland',
      genres: ['Ambient', 'Downtempo'],
      bio: 'An ambient project known for slow-building analog synth textures.',
      albums: [
        { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026, coverThumbUrl: '', coverUrl: '/artwork/abc/full.jpg' },
      ],
    };

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockDetail } as Response));

    const detail = await fetchArtistDetail(5);
    expect(detail).toEqual({
      id: 5,
      name: 'Solaris',
      photoUrl: 'http://localhost:8080/artwork/solaris/full.jpg',
      formedYear: 2019,
      country: 'Iceland',
      genres: ['Ambient', 'Downtempo'],
      bio: 'An ambient project known for slow-building analog synth textures.',
      albums: [
        { id: 1, title: 'Midnight Sun', artistName: 'Solaris', year: 2026, coverUrl: 'http://localhost:8080/artwork/abc/full.jpg' },
      ],
    });
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/library/artists/5', {
      headers: { Authorization: 'Bearer secret_token' },
    });
  });

  // Regression coverage for issue #69: tapping an album card had no
  // onPress handler at all, so nothing happened. fetchAlbumTracks backs
  // the fix — a real per-album track listing, not a filter over whatever
  // page of fetchSongsPage happens to already be loaded.
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
        { id: 101, title: 'Cosmic Voyager', trackNumber: 1, durationSeconds: 255, format: 'flac', bitrateKbps: 940 },
        { id: 102, title: 'Digital Horizon', trackNumber: 2, durationSeconds: 210, format: 'mp3', bitrateKbps: 320 },
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
        format: 'flac',
        bitrateKbps: 940,
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
        format: 'mp3',
        bitrateKbps: 320,
      },
    ]);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/library/albums/1', {
      headers: { Authorization: 'Bearer secret_token' },
    });
  });
});

describe('Playlists API Client (TDR 028)', () => {
  const creds = { serverUrl: 'http://localhost:8080', pairingToken: 'secret_token' };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(connection.getServerCredentials).mockResolvedValue(creds);
  });

  it('fetches a page of playlists and resolves cover URLs against the server origin', async () => {
    const mockPage = {
      items: [{ id: 1, name: 'Late Night Drive', trackCount: 2, coverUrls: ['/artwork/a/thumb.jpg', ''] }],
      page: 1,
      pageSize: 30,
      totalCount: 1,
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockPage } as Response));

    const page = await fetchPlaylistsPage({ sort: 'name' });
    expect(page.items).toEqual([
      { id: 1, name: 'Late Night Drive', trackCount: 2, coverUrls: ['http://localhost:8080/artwork/a/thumb.jpg', ''] },
    ]);
    expect(fetch).toHaveBeenCalledWith(
      'http://localhost:8080/api/playlists?page=1&pageSize=30&sort=name',
      { headers: { Authorization: 'Bearer secret_token' } },
    );
  });

  it('creates a playlist by POSTing its name', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 3, name: 'Study Session', trackCount: 0, coverUrls: [] }),
      } as Response),
    );

    const playlist = await createPlaylist('Study Session');
    expect(playlist).toEqual({ id: 3, name: 'Study Session', trackCount: 0, coverUrls: [] });
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists', {
      method: 'POST',
      headers: { Authorization: 'Bearer secret_token', 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'Study Session' }),
    });
  });

  it('fetches a playlist detail and maps its tracks, addressed by playlistTrackId not trackId', async () => {
    const mockDetail = {
      id: 1,
      name: 'Late Night Drive',
      trackCount: 1,
      coverUrls: [],
      tracks: [
        {
          playlistTrackId: 55,
          trackId: 101,
          title: 'Cosmic Voyager',
          artistName: 'Solaris',
          albumTitle: 'Midnight Sun',
          albumCoverThumbUrl: '/artwork/abc/thumb.jpg',
          durationSeconds: 255,
          format: 'flac',
          bitrateKbps: 940,
        },
      ],
    };
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => mockDetail } as Response));

    const detail = await fetchPlaylistDetail(1);
    expect(detail.tracks).toEqual([
      {
        id: 101,
        playlistTrackId: 55,
        title: 'Cosmic Voyager',
        artistName: 'Solaris',
        albumTitle: 'Midnight Sun',
        durationSeconds: 255,
        streamUrl: 'http://localhost:8080/api/library/songs/101/stream',
        coverUrl: 'http://localhost:8080/artwork/abc/thumb.jpg',
        format: 'flac',
        bitrateKbps: 940,
      },
    ]);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1', {
      headers: { Authorization: 'Bearer secret_token' },
    });
  });

  it('renames a playlist via PATCH', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 1, name: 'Sunday Coffee', trackCount: 0, coverUrls: [], tracks: [] }),
      } as Response),
    );

    const detail = await renamePlaylist(1, 'Sunday Coffee');
    expect(detail.name).toBe('Sunday Coffee');
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1', {
      method: 'PATCH',
      headers: { Authorization: 'Bearer secret_token', 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: 'Sunday Coffee' }),
    });
  });

  it('deletes a playlist and throws if the request fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true } as Response));
    await deletePlaylist(1);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1', {
      method: 'DELETE',
      headers: { Authorization: 'Bearer secret_token' },
    });

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, statusText: 'Not Found' } as Response));
    await expect(deletePlaylist(999)).rejects.toThrow('Request to /api/playlists/999 failed: Not Found');
  });

  it('adds a track to a playlist via POST .../tracks', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true } as Response));
    await addTrackToPlaylist(1, 101);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1/tracks', {
      method: 'POST',
      headers: { Authorization: 'Bearer secret_token', 'Content-Type': 'application/json' },
      body: JSON.stringify({ trackId: 101 }),
    });
  });

  it('removes one playlist entry by its playlistTrackId, not trackId, and returns the fresh detail', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 1, name: 'Late Night Drive', trackCount: 0, coverUrls: [], tracks: [] }),
      } as Response),
    );
    const detail = await removePlaylistTrack(1, 55);
    expect(detail.trackCount).toBe(0);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1/tracks/55', {
      method: 'DELETE',
      headers: { Authorization: 'Bearer secret_token' },
    });
  });

  it('reorders a playlist track and returns the fresh detail', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ id: 1, name: 'Late Night Drive', trackCount: 0, coverUrls: [], tracks: [] }),
      } as Response),
    );

    await reorderPlaylistTracks(1, 55, 2);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/playlists/1/tracks/reorder', {
      method: 'PATCH',
      headers: { Authorization: 'Bearer secret_token', 'Content-Type': 'application/json' },
      body: JSON.stringify({ playlistTrackId: 55, toIndex: 2 }),
    });
  });

  it("fetches the playlists containing a track, for the add-to-playlist sheet's pre-checked state", async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => [{ id: 1, name: 'Late Night Drive', trackCount: 1, coverUrls: [] }],
      } as Response),
    );

    const playlists = await fetchPlaylistsContainingTrack(101);
    expect(playlists).toEqual([{ id: 1, name: 'Late Night Drive', trackCount: 1, coverUrls: [] }]);
    expect(fetch).toHaveBeenCalledWith('http://localhost:8080/api/library/songs/101/playlists', {
      headers: { Authorization: 'Bearer secret_token' },
    });
  });
});
