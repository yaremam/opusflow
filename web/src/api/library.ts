export type ImportStatus = 'copying' | 'complete' | 'failed'

export interface FileError {
  path: string
  error: string
}

// Library is a named root folder opusflow organizes imports into (TDR 006)
// — created and managed entirely within the app, not an environment
// variable.
export interface Library {
  id: number
  name: string
  rootPath: string
  trackCount: number
  createdAt: string
}

export interface Import {
  id: number
  libraryId: number
  sourceDescription: string
  status: ImportStatus
  filesProcessed: number
  filesTotal: number
  error?: string
  trackCount: number
  fileErrors: FileError[]
  createdAt: string
}

export interface Entry {
  name: string
  path: string
}

// PlanTrack/PlanAlbum/Plan mirror the backend's organize.Track/Album/Plan —
// the review-plan shape a source directory or upload resolves into. Named
// distinctly from the catalog's Album/Song types below since a plan track
// isn't a catalog row yet (nothing has been copied).
export interface PlanTrack {
  sourcePath: string
  trackNumber: number
  title: string
  destPath: string
  conflict: boolean
  overwrite: boolean
}

export interface PlanAlbum {
  artist: string
  album: string
  year: number
  tracks: PlanTrack[]
}

export interface Plan {
  albums: PlanAlbum[]
}

// ValidationError reports one track that isn't ready to copy — missing (a
// list of required field names) and/or conflict (an unresolved destination
// clash). Indexes into the Plan the server was handed, so a caller can map
// an error straight back to the row it came from.
export interface ValidationError {
  albumIndex: number
  trackIndex: number
  missing?: string[]
  conflict?: boolean
}

export interface PlanResponse {
  plan: Plan
  errors: ValidationError[]
}

export interface Artist {
  id: number
  name: string
  albumCount: number
  trackCount: number
  createdAt: string

  // Populated by the background enrichment job (TDR 003) — all start
  // zero-valued until it runs. An empty photoUrl/photoThumbUrl means "show
  // the placeholder tile", not "still loading"; empty genres/bio mean that
  // section just isn't rendered.
  photoThumbUrl: string
  photoUrl: string
  formedYear: number
  country: string
  genres: string[]
  bio: string
  bioSourceUrl: string
}

export interface Album {
  id: number
  title: string
  artistId: number
  artistName: string
  year: number
  trackCount: number
  createdAt: string

  coverThumbUrl: string
  coverUrl: string
  label: string
  country: string
  genres: string[]
  description: string
  descriptionSourceUrl: string
}

export interface Song {
  id: number
  title: string
  artistId: number
  artistName: string
  albumId: number
  albumTitle: string
  albumCoverThumbUrl: string
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

function postJSON<T>(path: string, body: unknown): Promise<T> {
  return request(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

export function listLibraries(): Promise<Library[]> {
  return request('/api/libraries')
}

export function createLibrary(name: string, rootPath: string): Promise<Library> {
  return postJSON('/api/libraries', { name, rootPath })
}

export function deleteLibrary(id: number, deleteFiles: boolean): Promise<void> {
  return request(`/api/libraries/${id}?deleteFiles=${deleteFiles}`, { method: 'DELETE' })
}

export function browse(path: string): Promise<Entry[]> {
  return request(`/api/imports/browse?path=${encodeURIComponent(path)}`)
}

export function buildPlan(libraryId: number, sourceDir: string): Promise<PlanResponse> {
  return postJSON('/api/imports/plan', { libraryId, sourceDir })
}

export function validatePlan(libraryId: number, plan: Plan): Promise<PlanResponse> {
  return postJSON('/api/imports/plan/validate', { libraryId, plan })
}

export interface UploadFile {
  file: File
  relativePath: string
}

export interface UploadProgress {
  loadedBytes: number
  totalBytes: number
}

// uploadImport POSTs every file as one multipart request, preserving each
// file's relative folder path (webkitRelativePath, for a folder picked via
// <input webkitdirectory>) as the multipart part's filename, so the backend
// stages them into the same directory shape a browsed server folder would
// have had. Uses XMLHttpRequest rather than fetch because progress events
// on a request body aren't otherwise observable — onProgress reports
// combined bytes across the whole upload, not per file; the caller derives
// individual file progress from each file's byte offset within the request.
export function uploadImport(
  libraryId: number,
  files: UploadFile[],
  onProgress: (progress: UploadProgress) => void,
): Promise<PlanResponse> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('libraryId', String(libraryId))
    for (const f of files) {
      form.append('files', f.file, f.relativePath)
    }

    const xhr = new XMLHttpRequest()
    xhr.open('POST', '/api/imports/upload')
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress({ loadedBytes: e.loaded, totalBytes: e.total })
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as PlanResponse)
        } catch {
          reject(new Error('Server returned an invalid response'))
        }
      } else {
        reject(new Error(xhr.responseText.trim() || `Request failed with status ${xhr.status}`))
      }
    }
    xhr.onerror = () => reject(new Error('Upload failed'))
    xhr.send(form)
  })
}

export interface ConfirmResult {
  import?: Import
  errors?: ValidationError[]
}

// confirmImport handles its own response status rather than going through
// request(): a 422 (plan still isn't ready) is an expected outcome the
// review screen redraws around, not an exception — only an actual
// network/server failure should throw.
export async function confirmImport(libraryId: number, sourceDescription: string, plan: Plan): Promise<ConfirmResult> {
  const res = await fetch('/api/imports', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ libraryId, sourceDescription, plan }),
  })
  if (res.status === 422) {
    const body = (await res.json()) as { errors: ValidationError[] }
    return { errors: body.errors }
  }
  if (!res.ok) {
    const text = (await res.text()).trim()
    throw new Error(text || `Request failed with status ${res.status}`)
  }
  return { import: (await res.json()) as Import }
}

export function listImports(): Promise<Import[]> {
  return request('/api/imports')
}

export function getImport(id: number): Promise<Import> {
  return request(`/api/imports/${id}`)
}

export function deleteArtist(id: number, deleteFiles: boolean): Promise<void> {
  return request(`/api/library/artists/${id}?deleteFiles=${deleteFiles}`, { method: 'DELETE' })
}

export function deleteAlbum(id: number, deleteFiles: boolean): Promise<void> {
  return request(`/api/library/albums/${id}?deleteFiles=${deleteFiles}`, { method: 'DELETE' })
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
