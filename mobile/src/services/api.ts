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
  albumId?: number;
  durationSeconds: number;
  streamUrl: string;
  coverUrl?: string;
  localCoverUrl?: string;
  isOffline?: boolean;
}

// Page mirrors the backend's library.Page[T] (backend/internal/library/catalog.go)
// — every /api/library/* list endpoint returns one of these, not a bare array.
interface Page<T> {
  items: T[];
  page: number;
  pageSize: number;
  totalCount: number;
}

// Backend's real /api/library/albums row shape (backend/internal/library/catalog.go
// Album, also web/src/api/library.ts's Album) — kept separate from this file's own
// Album above, which is the narrower shape the app's screens actually consume.
interface BackendAlbum {
  id: number;
  title: string;
  artistName: string;
  year: number;
  coverThumbUrl: string;
  coverUrl: string;
}

// Backend's real /api/library/songs row shape (backend/internal/library/catalog.go
// Song).
interface BackendSong {
  id: number;
  title: string;
  artistName: string;
  albumId: number;
  albumTitle: string;
  albumCoverThumbUrl: string;
  durationSeconds: number;
}

// maxPageSize matches the backend's own library.maxPageSize
// (backend/internal/library/service.go) — the most a single list call can
// return; there's no mobile pagination UI yet, so every fetch asks for as
// much as the interface allows in one page rather than silently truncating
// to the backend's 30-item default.
const maxPageSize = 100;

async function fetchFromLibrary<T>(path: string, searchQuery: string): Promise<Page<T>> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const url = new URL(`${creds.serverUrl}${path}`);
  url.searchParams.set('pageSize', String(maxPageSize));
  if (searchQuery) url.searchParams.set('q', searchQuery);

  const res = await fetch(url.toString(), {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });

  if (!res.ok) throw new Error(`Request to ${path} failed: ${res.statusText}`);
  return (await res.json()) as Page<T>;
}

// absoluteArtworkUrl resolves a relative artwork path (e.g. "/artwork/xyz.jpg",
// or "" when nothing's been found yet — see ArtStatus in web/src/api/library.ts)
// against the paired server's own origin — the backend always returns these
// relative, since the web app that path convention was designed for is served
// from the same origin as the API itself (web/src/api/library.ts's request()
// hits plain relative paths for the same reason). Mobile has no such origin to
// inherit, so it has to do this prefixing itself.
function absoluteArtworkUrl(serverUrl: string, relativePath: string): string | undefined {
  return relativePath ? `${serverUrl}${relativePath}` : undefined;
}

export async function fetchCatalogAlbums(searchQuery: string = ''): Promise<Album[]> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const page = await fetchFromLibrary<BackendAlbum>('/api/library/albums', searchQuery);
  return page.items.map((album) => ({
    id: album.id,
    title: album.title,
    artistName: album.artistName,
    year: album.year,
    coverUrl: absoluteArtworkUrl(creds.serverUrl, album.coverUrl || album.coverThumbUrl),
  }));
}

export async function fetchCatalogTracks(searchQuery: string = ''): Promise<Track[]> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const page = await fetchFromLibrary<BackendSong>('/api/library/songs', searchQuery);
  return page.items.map((t) => ({
    id: t.id,
    title: t.title,
    artistName: t.artistName,
    albumTitle: t.albumTitle,
    albumId: t.albumId,
    durationSeconds: t.durationSeconds,
    streamUrl: `${creds.serverUrl}/api/library/songs/${t.id}/stream`,
    coverUrl: absoluteArtworkUrl(creds.serverUrl, t.albumCoverThumbUrl),
  }));
}
