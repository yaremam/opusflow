export type DirectoryStatus = 'scanning' | 'complete' | 'failed'

export interface FileError {
  path: string
  error: string
}

export interface LibraryDirectory {
  id: number
  root: string
  path: string
  status: DirectoryStatus
  filesProcessed: number
  filesTotal: number
  error?: string
  trackCount: number
  fileErrors: FileError[]
  createdAt: string
}

export interface RootInfo {
  path: string
}

export interface Entry {
  name: string
  path: string
}

export interface Artist {
  id: number
  name: string
  albumCount: number
  trackCount: number
  createdAt: string
}

export interface Album {
  id: number
  title: string
  artistId: number
  artistName: string
  year: number
  trackCount: number
  createdAt: string
}

export interface Song {
  id: number
  title: string
  artistId: number
  artistName: string
  albumId: number
  albumTitle: string
  trackNumber: number
  year: number
  genre: string
  durationSeconds: number
  createdAt: string
}

export interface AlbumTrack {
  id: number
  title: string
  trackNumber: number
  durationSeconds: number
}

export interface ArtistDetail extends Artist {
  albums: Album[]
}

export interface AlbumDetail extends Album {
  tracks: AlbumTrack[]
}

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  totalCount: number
}

export interface ListParams {
  page?: number
  pageSize?: number
  sort?: 'recent' | 'name'
  genre?: string
  year?: number
  q?: string
}

// formatDuration renders a track length as "m:ss" (or "h:mm:ss" past an
// hour), the convention used throughout the catalog UI.
export function formatDuration(totalSeconds: number): string {
  const h = Math.floor(totalSeconds / 3600)
  const m = Math.floor((totalSeconds % 3600) / 60)
  const s = totalSeconds % 60
  const mm = h > 0 ? String(m).padStart(2, '0') : String(m)
  const ss = String(s).padStart(2, '0')
  return h > 0 ? `${h}:${mm}:${ss}` : `${mm}:${ss}`
}

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

// The last non-empty path segment (e.g. "/mnt/music" -> "music") — used as
// a short "which mount is this" identifier, not the full configured path.
export function rootLabel(root: string): string {
  return root.split('/').filter(Boolean).pop() ?? root
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init)
  if (!res.ok) {
    const text = (await res.text()).trim()
    throw new Error(text || `Request failed with status ${res.status}`)
  }
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

export function listRoots(): Promise<RootInfo[]> {
  return request('/api/library/roots')
}

export function browse(path: string): Promise<Entry[]> {
  return request(`/api/library/browse?path=${encodeURIComponent(path)}`)
}

export function listDirectories(): Promise<LibraryDirectory[]> {
  return request('/api/library/directories')
}

export function addDirectory(path: string): Promise<LibraryDirectory> {
  return request('/api/library/directories', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
}

export function removeDirectory(id: number): Promise<void> {
  return request(`/api/library/directories/${id}`, { method: 'DELETE' })
}

function listParams(params: ListParams): string {
  const q = new URLSearchParams()
  if (params.page) q.set('page', String(params.page))
  if (params.pageSize) q.set('pageSize', String(params.pageSize))
  if (params.sort) q.set('sort', params.sort)
  if (params.genre) q.set('genre', params.genre)
  if (params.year) q.set('year', String(params.year))
  if (params.q) q.set('q', params.q)
  const s = q.toString()
  return s ? `?${s}` : ''
}

export function listArtists(params: ListParams = {}): Promise<Page<Artist>> {
  return request(`/api/library/artists${listParams(params)}`)
}

export function getArtist(id: number): Promise<ArtistDetail> {
  return request(`/api/library/artists/${id}`)
}

export function listAlbums(params: ListParams = {}): Promise<Page<Album>> {
  return request(`/api/library/albums${listParams(params)}`)
}

export function getAlbum(id: number): Promise<AlbumDetail> {
  return request(`/api/library/albums/${id}`)
}

export function listSongs(params: ListParams = {}): Promise<Page<Song>> {
  return request(`/api/library/songs${listParams(params)}`)
}
