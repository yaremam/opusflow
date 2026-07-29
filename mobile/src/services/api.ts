import { getServerCredentials } from './connection';

export interface Artist {
  id: number;
  name: string;
  photoUrl?: string;
}

export interface Album {
  id: number;
  title: string;
  artistName: string;
  year?: number;
  coverUrl?: string;
}

export interface Track {
  id: number;
  title: string;
  artistName: string;
  albumTitle: string;
  durationSeconds: number;
  streamUrl: string;
  isOffline?: boolean;
}

export async function fetchCatalogAlbums(searchQuery: string = ''): Promise<Album[]> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const url = new URL(`${creds.serverUrl}/api/catalog/albums`);
  if (searchQuery) url.searchParams.set('q', searchQuery);

  const res = await fetch(url.toString(), {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });

  if (!res.ok) throw new Error(`Failed to fetch albums: ${res.statusText}`);
  return res.json();
}

export async function fetchCatalogTracks(searchQuery: string = ''): Promise<Track[]> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const url = new URL(`${creds.serverUrl}/api/catalog/songs`);
  if (searchQuery) url.searchParams.set('q', searchQuery);

  const res = await fetch(url.toString(), {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });

  if (!res.ok) throw new Error(`Failed to fetch tracks: ${res.statusText}`);
  const tracks: Track[] = await res.json();

  return tracks.map((t) => ({
    ...t,
    streamUrl: `${creds.serverUrl}/api/stream/${t.id}`,
  }));
}
