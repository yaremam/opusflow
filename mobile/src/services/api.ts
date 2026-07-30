import { getServerCredentials } from './connection';

export interface Artist {
  id: number;
  name: string;
  albumCount: number;
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
  // format/bitrateKbps back the Player screen's quality badge (backlog/027)
  // — bitrateKbps is 0 for a track scanned before file_size_bytes existed
  // (see backend/internal/library/catalog.go's bitrateKbps), meaning
  // "unknown," not zero-quality.
  format: string;
  bitrateKbps: number;
}

// Page mirrors the backend's library.Page[T] (backend/internal/library/catalog.go)
// — every /api/library/* list endpoint returns one of these, not a bare array.
export interface Page<T> {
  items: T[];
  page: number;
  pageSize: number;
  totalCount: number;
}

// ListParams mirrors web's own ListParams (web/src/api/library.ts) — the
// same search/sort/genre/year filter set (backlog/026 AC-2), against the
// same query parameters the backend's parseListOptions already reads.
export interface ListParams {
  page?: number;
  sort?: 'recent' | 'name';
  genre?: string;
  year?: number;
  q?: string;
}

// libraryPageSize is a fixed page size for infinite scroll (backlog/026
// AC-2) — mobile has no numbered-pager UI; each page just extends the
// list already on screen.
const libraryPageSize = 30;

// Backend's real /api/library/artists row shape (backend/internal/library/catalog.go
// Artist).
interface BackendArtist {
  id: number;
  name: string;
  albumCount: number;
  photoThumbUrl: string;
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
  format: string;
  bitrateKbps: number;
}

async function fetchPageFromLibrary<T>(path: string, params: ListParams): Promise<{ creds: NonNullable<Awaited<ReturnType<typeof getServerCredentials>>>; page: Page<T> }> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const url = new URL(`${creds.serverUrl}${path}`);
  url.searchParams.set('page', String(params.page ?? 1));
  url.searchParams.set('pageSize', String(libraryPageSize));
  if (params.sort) url.searchParams.set('sort', params.sort);
  if (params.genre) url.searchParams.set('genre', params.genre);
  if (params.year) url.searchParams.set('year', String(params.year));
  if (params.q) url.searchParams.set('q', params.q);

  const res = await fetch(url.toString(), {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });

  if (!res.ok) throw new Error(`Request to ${path} failed: ${res.statusText}`);
  return { creds, page: (await res.json()) as Page<T> };
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

function toAlbum(serverUrl: string, album: BackendAlbum): Album {
  return {
    id: album.id,
    title: album.title,
    artistName: album.artistName,
    year: album.year,
    coverUrl: absoluteArtworkUrl(serverUrl, album.coverUrl || album.coverThumbUrl),
  };
}

// fetchArtistsPage/fetchAlbumsPage/fetchSongsPage back the Library hub's
// three real lists (backlog/026 AC-2) — infinite scroll (each call is one
// more page) plus web's full search/sort/genre/year filter set, replacing
// the old featured-albums strip + single unpaginated track dump.
export async function fetchArtistsPage(params: ListParams = {}): Promise<Page<Artist>> {
  const { creds, page } = await fetchPageFromLibrary<BackendArtist>('/api/library/artists', params);
  return {
    ...page,
    items: page.items.map((artist) => ({
      id: artist.id,
      name: artist.name,
      albumCount: artist.albumCount,
      photoUrl: absoluteArtworkUrl(creds.serverUrl, artist.photoThumbUrl),
    })),
  };
}

export async function fetchAlbumsPage(params: ListParams = {}): Promise<Page<Album>> {
  const { creds, page } = await fetchPageFromLibrary<BackendAlbum>('/api/library/albums', params);
  return { ...page, items: page.items.map((album) => toAlbum(creds.serverUrl, album)) };
}

// ArtistDetail's real response shape (backend/internal/library/catalog.go
// ArtistDetail) — only the bio/facts + albums fields mobile's Artist
// Detail screen needs (backlog/026 AC-3), matching web's ArtistDetailPage
// content rather than a stripped-down version.
export interface ArtistDetail {
  id: number;
  name: string;
  photoUrl?: string;
  formedYear?: number;
  country?: string;
  genres: string[];
  bio?: string;
  albums: Album[];
}

interface BackendArtistDetail {
  id: number;
  name: string;
  photoUrl: string;
  formedYear: number;
  country: string;
  genres: string[];
  bio: string;
  albums: BackendAlbum[];
}

export async function fetchArtistDetail(id: number): Promise<ArtistDetail> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const res = await fetch(`${creds.serverUrl}/api/library/artists/${id}`, {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });
  if (!res.ok) throw new Error(`Request to /api/library/artists/${id} failed: ${res.statusText}`);
  const detail = (await res.json()) as BackendArtistDetail;

  return {
    id: detail.id,
    name: detail.name,
    photoUrl: absoluteArtworkUrl(creds.serverUrl, detail.photoUrl),
    formedYear: detail.formedYear || undefined,
    country: detail.country || undefined,
    genres: detail.genres,
    bio: detail.bio || undefined,
    albums: detail.albums.map((album) => toAlbum(creds.serverUrl, album)),
  };
}

// AlbumDetail's real response shape (backend/internal/library/catalog.go
// AlbumDetail) — only the track-listing fields this needs, not the full
// Album/covers/banner shape web's AlbumDetailPage consumes.
interface AlbumDetailTracks {
  tracks: {
    id: number;
    title: string;
    trackNumber: number;
    durationSeconds: number;
    format: string;
    bitrateKbps: number;
  }[];
}

// fetchAlbumTracks backs "tap an album to play it" (issue #69) — a real
// per-album track listing via GET /api/library/albums/{id}, not a
// client-side filter of whatever page of fetchSongsPage happens to be
// loaded, which wouldn't reliably contain every track once a library
// grows past one page.
export async function fetchAlbumTracks(album: Album): Promise<Track[]> {
  const creds = await getServerCredentials();
  if (!creds) throw new Error('No server credentials saved.');

  const res = await fetch(`${creds.serverUrl}/api/library/albums/${album.id}`, {
    headers: { Authorization: `Bearer ${creds.pairingToken}` },
  });
  if (!res.ok) throw new Error(`Request to /api/library/albums/${album.id} failed: ${res.statusText}`);
  const detail = (await res.json()) as AlbumDetailTracks;

  return detail.tracks.map((t) => ({
    id: t.id,
    title: t.title,
    artistName: album.artistName,
    albumTitle: album.title,
    albumId: album.id,
    durationSeconds: t.durationSeconds,
    streamUrl: `${creds.serverUrl}/api/library/songs/${t.id}/stream`,
    coverUrl: album.coverUrl,
    format: t.format,
    bitrateKbps: t.bitrateKbps,
  }));
}

export async function fetchSongsPage(params: ListParams = {}): Promise<Page<Track>> {
  const { creds, page } = await fetchPageFromLibrary<BackendSong>('/api/library/songs', params);
  return {
    ...page,
    items: page.items.map((t) => ({
      id: t.id,
      title: t.title,
      artistName: t.artistName,
      albumTitle: t.albumTitle,
      albumId: t.albumId,
      durationSeconds: t.durationSeconds,
      streamUrl: `${creds.serverUrl}/api/library/songs/${t.id}/stream`,
      coverUrl: absoluteArtworkUrl(creds.serverUrl, t.albumCoverThumbUrl),
      format: t.format,
      bitrateKbps: t.bitrateKbps,
    })),
  };
}
