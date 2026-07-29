import { getToken, notifyUnauthorized } from '../auth'

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
  // hasCorrectionFile is set for a WavPack (.wv) track with a matching
  // .wvc hybrid-mode correction file alongside it (TDR 013) — that file
  // rides along wherever this track goes, sharing its conflict/overwrite
  // status rather than getting a review row of its own.
  hasCorrectionFile?: boolean
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

// ArtStatus is an artist/album's photo/cover lookup state (TDR 003). A
// badge/pill (TDR 007) only ever renders for not_found/failed, and only
// when there's no image to fall back on instead — see hasArtProblem below.
export type ArtStatus = 'pending' | 'found' | 'not_found' | 'failed'

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
  artStatus: ArtStatus
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
  artStatus: ArtStatus
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
  albumArtStatus: ArtStatus
  trackNumber: number
  year: number
  genre: string
  durationSeconds: number
  createdAt: string
  // format is the track's file extension (TDR 015), lowercased and
  // dot-stripped ("mp3"/"flac"/"m4a"/"ogg"/"wv") — used to disable
  // playback for formats no browser can decode (WavPack).
  format: string
}

// hasArtProblem is the one rule every badge/pill in the app follows: show
// something is wrong only when there's genuinely no image to show instead
// — a not_found/failed status next to a still-good (or newly-uploaded)
// image is not a problem worth flagging (TDR 007 AC-2/AC-3).
export function hasArtProblem(status: ArtStatus, thumbUrl: string): boolean {
  return (status === 'not_found' || status === 'failed') && thumbUrl === ''
}

export interface AlbumTrack {
  id: number
  title: string
  trackNumber: number
  durationSeconds: number
  format: string
}

// ArtistPhoto/AlbumCover are one image in an artist's/album's gallery (TDR
// 014) — exactly one per entity has isPrimary set, which is the one
// reflected in photoThumbUrl/coverThumbUrl above for list views and grid
// tiles that only have room for one image. isBanner is TDR 016's second,
// independent flag — at most one image also has it set, and it need not
// be the same image as the primary one.
export interface ArtistPhoto {
  id: number
  thumbUrl: string
  fullUrl: string
  source: string
  isPrimary: boolean
  isBanner: boolean
  createdAt: string
}

export interface AlbumCover {
  id: number
  thumbUrl: string
  fullUrl: string
  source: string
  pictureType?: string
  isPrimary: boolean
  isBanner: boolean
  createdAt: string
}

// bannerUrl (TDR 016) is the detail page header's banner image — the
// gallery image flagged isBanner, falling back to the primary image when
// none is flagged yet (never blank while the gallery has any image).
export interface ArtistDetail extends Artist {
  albums: Album[]
  photos: ArtistPhoto[]
  bannerUrl: string
}

export interface AlbumDetail extends Album {
  tracks: AlbumTrack[]
  covers: AlbumCover[]
  bannerUrl: string
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
  const headers = new Headers(init?.headers)
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(path, { ...init, headers })
  if (res.status === 401) {
    notifyUnauthorized()
    throw new Error('Session expired — please unlock again.')
  }
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

// ApiToken is one row of the "Paired Devices" list (TDR 022 AC-5) —
// never the token itself, which is only ever known once, at creation.
export interface ApiToken {
  id: number
  name: string
  createdAt: string
  lastUsedAt: string | null
}

// CreatedApiToken is what POST /api/tokens returns — the one and only
// response carrying the plaintext token, shown as text and a QR code
// (serverUrl + token) for scanning from the mobile app.
export interface CreatedApiToken extends ApiToken {
  token: string
}

export function listTokens(): Promise<ApiToken[]> {
  return request('/api/tokens')
}

export function createToken(name: string): Promise<CreatedApiToken> {
  return postJSON('/api/tokens', { name })
}

export function deleteToken(id: number): Promise<void> {
  return request(`/api/tokens/${id}`, { method: 'DELETE' })
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

// retryArtistArt/retryAlbumArt queue a fresh art lookup (TDR 007) and wake
// the background enrichment job immediately rather than waiting for the
// next import. The response reflects the reset (status back to pending)
// right away — the caller polls get{Artist,Album} afterward for the
// eventual outcome, see pollForArtResolution below.
export function retryArtistArt(id: number): Promise<Artist> {
  return postJSON(`/api/library/artists/${id}/art/retry`, {})
}

export function retryAlbumArt(id: number): Promise<Album> {
  return postJSON(`/api/library/albums/${id}/art/retry`, {})
}

function uploadArt<T>(path: string, file: File): Promise<T> {
  const form = new FormData()
  form.append('image', file)
  return request(path, { method: 'POST', body: form })
}

// uploadArtistArt/uploadAlbumArt add a manually-chosen image to the
// gallery (TDR 014 AC-3: upload always adds, never replaces) — synchronous
// (no polling needed): the response already reflects the new gallery,
// including the freshly-added photo/cover.
export function uploadArtistArt(id: number, file: File): Promise<ArtistDetail> {
  return uploadArt(`/api/library/artists/${id}/art`, file)
}

export function uploadAlbumArt(id: number, file: File): Promise<AlbumDetail> {
  return uploadArt(`/api/library/albums/${id}/art`, file)
}

// setArtistPrimaryPhoto/setAlbumPrimaryCover mark one gallery image as the
// one shown in list views and grid tiles (AC-2); deleteArtistPhoto/
// deleteAlbumCover remove one image, optionally deleting its file from
// disk too (AC-4, same explicit keep-vs-delete-file choice as
// deleteArtist/deleteAlbum). Both return the entity's fresh detail
// (gallery included) so the gallery UI can re-render from one response.
export function setArtistPrimaryPhoto(artistId: number, photoId: number): Promise<ArtistDetail> {
  return postJSON(`/api/library/artists/${artistId}/photos/${photoId}/primary`, {})
}

// setArtistBannerPhoto/setAlbumBannerCover mark one gallery image as the
// detail page header's banner (TDR 016) — independent of the primary
// image set above.
export function setArtistBannerPhoto(artistId: number, photoId: number): Promise<ArtistDetail> {
  return postJSON(`/api/library/artists/${artistId}/photos/${photoId}/banner`, {})
}

export function deleteArtistPhoto(artistId: number, photoId: number, deleteFile: boolean): Promise<ArtistDetail> {
  return request(`/api/library/artists/${artistId}/photos/${photoId}?deleteFile=${deleteFile}`, { method: 'DELETE' })
}

export function setAlbumPrimaryCover(albumId: number, coverId: number): Promise<AlbumDetail> {
  return postJSON(`/api/library/albums/${albumId}/covers/${coverId}/primary`, {})
}

export function setAlbumBannerCover(albumId: number, coverId: number): Promise<AlbumDetail> {
  return postJSON(`/api/library/albums/${albumId}/covers/${coverId}/banner`, {})
}

export function deleteAlbumCover(albumId: number, coverId: number, deleteFile: boolean): Promise<AlbumDetail> {
  return request(`/api/library/albums/${albumId}/covers/${coverId}?deleteFile=${deleteFile}`, { method: 'DELETE' })
}

// pollForArtResolution re-fetches an artist/album every 2s for up to ~30s
// after a retry, stopping as soon as its status leaves "pending" (AC-6).
// Generic over Artist/Album since both follow the identical shape.
export async function pollForArtResolution<T extends { artStatus: ArtStatus }>(
  fetchOne: () => Promise<T>,
): Promise<T> {
  const deadline = Date.now() + 30_000
  let latest = await fetchOne()
  while (latest.artStatus === 'pending' && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 2000))
    latest = await fetchOne()
  }
  return latest
}

export interface About {
  version: string
  buildDate: string
}

// getAbout fetches the running build's identity for the About page (TDR
// 008) — version is a `git describe` string, buildDate a UTC timestamp;
// both come from the backend rather than being baked into the web bundle,
// so the same static assets report correctly regardless of which backend
// build actually served them.
export function getAbout(): Promise<About> {
  return request('/api/about')
}

export interface Config {
  dataDir: string
}

// getConfig fetches server-side config the picker needs before it can
// browse anything — currently just dataDir (empty means unrestricted),
// telling SourceFolderPicker where to start instead of defaulting to "/"
// and immediately hitting a "path is outside the configured data
// directory" error when a DATA_DIR is set.
export function getConfig(): Promise<Config> {
  return request('/api/config')
}

export function browse(path: string): Promise<Entry[]> {
  return request(`/api/imports/browse?path=${encodeURIComponent(path)}`)
}

export function createFolder(parentPath: string, name: string): Promise<Entry> {
  return postJSON('/api/imports/browse/folders', { parentPath, name })
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

// mergeArtist/mergeAlbum fold id — every album, track, and gallery image
// it has — into intoId, then remove id (issue #31's manual merge tool).
// Returns intoId's fresh detail, since id's own page no longer exists
// once this resolves.
export function mergeArtist(id: number, intoId: number): Promise<ArtistDetail> {
  return postJSON(`/api/library/artists/${id}/merge`, { intoId })
}

export function mergeAlbum(id: number, intoId: number): Promise<AlbumDetail> {
  return postJSON(`/api/library/albums/${id}/merge`, { intoId })
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

// streamURL builds the audio-streaming URL for a track (TDR 015) — handed
// straight to an <audio> element's src, never fetched directly, so the
// browser can issue its own range requests against it.
export function streamURL(id: number): string {
  return `/api/library/songs/${id}/stream`
}

// ArtistMatch/ReleaseGroupMatch/ReleaseMatch/MetadataTrack mirror the
// backend's enrich.ArtistMatch/ReleaseGroupMatch/ReleaseMatch/Track — the
// interactive "look up metadata" flow's search-result shapes (TDR 012),
// distinct from the background enrichment system's by-ID lookups.
export interface ArtistMatch {
  mbid: string
  name: string
  disambiguation?: string
}

export interface ReleaseGroupMatch {
  mbid: string
  title: string
  firstReleaseYear?: number
}

export interface ReleaseMatch {
  mbid: string
  country?: string
  date?: string
  trackCount: number
}

export interface MetadataTrack {
  position: number
  title: string
}

export function searchArtists(query: string): Promise<ArtistMatch[]> {
  return request(`/api/metadata/artists?q=${encodeURIComponent(query)}`)
}

export function artistReleaseGroups(artistMBID: string): Promise<ReleaseGroupMatch[]> {
  return request(`/api/metadata/artists/${encodeURIComponent(artistMBID)}/release-groups`)
}

export function releaseGroupReleases(releaseGroupMBID: string): Promise<ReleaseMatch[]> {
  return request(`/api/metadata/release-groups/${encodeURIComponent(releaseGroupMBID)}/releases`)
}

export function releaseTracks(releaseMBID: string): Promise<MetadataTrack[]> {
  return request(`/api/metadata/releases/${encodeURIComponent(releaseMBID)}/tracks`)
}
