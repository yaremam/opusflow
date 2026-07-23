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
