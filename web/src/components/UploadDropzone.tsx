import { useMemo, useRef, useState } from 'react'
import { errorMessage, uploadImport, type PlanResponse, type UploadFile, type UploadProgress } from '../api/library'
import './UploadDropzone.css'

interface UploadDropzoneProps {
  libraryId: number
  onBack: () => void
  onUploaded: (result: PlanResponse) => void
}

function isFileEntry(entry: FileSystemEntry): entry is FileSystemFileEntry {
  return entry.isFile
}
function isDirectoryEntry(entry: FileSystemEntry): entry is FileSystemDirectoryEntry {
  return entry.isDirectory
}

// readEntry recursively walks a dropped file or folder into a flat list of
// UploadFile — the File System Entry API a drag-and-drop gives access to,
// unlike a plain <input type="file">, requires manually recursing into
// directories (readEntries only returns one batch at a time, hence the
// loop) rather than handing back every descendant file up front.
async function readEntry(entry: FileSystemEntry, prefix: string): Promise<UploadFile[]> {
  if (isFileEntry(entry)) {
    const file = await new Promise<File>((resolve, reject) => entry.file(resolve, reject))
    return [{ file, relativePath: `${prefix}${file.name}` }]
  }
  if (isDirectoryEntry(entry)) {
    const reader = entry.createReader()
    const children: FileSystemEntry[] = []
    for (;;) {
      const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject))
      if (batch.length === 0) break
      children.push(...batch)
    }
    const nested = await Promise.all(children.map((child) => readEntry(child, `${prefix}${entry.name}/`)))
    return nested.flat()
  }
  return []
}

async function filesFromDataTransfer(items: DataTransferItemList): Promise<UploadFile[]> {
  const entries: FileSystemEntry[] = []
  for (let i = 0; i < items.length; i++) {
    const entry = items[i].webkitGetAsEntry()
    if (entry) entries.push(entry)
  }
  const nested = await Promise.all(entries.map((entry) => readEntry(entry, '')))
  return nested.flat()
}

function filesFromFileList(fileList: FileList): UploadFile[] {
  return Array.from(fileList).map((file) => ({
    file,
    relativePath: file.webkitRelativePath || file.name,
  }))
}

function formatBytes(bytes: number): string {
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(1)} KB`
  return `${bytes} B`
}

// UploadDropzone implements "upload from this device" (TDR 005): pick or
// drop a folder, watch it stage to the server, then move on once every file
// has arrived. Uses XMLHttpRequest via uploadImport for progress; a single
// request carries every file, so per-file progress below is derived from
// each file's byte offset within the whole upload rather than tracked
// independently.
export default function UploadDropzone({ libraryId, onBack, onUploaded }: UploadDropzoneProps) {
  const [files, setFiles] = useState<UploadFile[]>([])
  const [uploading, setUploading] = useState(false)
  const [progress, setProgress] = useState<UploadProgress | null>(null)
  const [result, setResult] = useState<PlanResponse | null>(null)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const offsets = useMemo(() => {
    let cumulative = 0
    return files.map((f) => {
      const start = cumulative
      cumulative += f.file.size
      return { start, end: cumulative }
    })
  }, [files])

  async function startUpload(selected: UploadFile[]) {
    if (selected.length === 0) return
    setFiles(selected)
    setResult(null)
    setUploadError(null)
    setUploading(true)
    try {
      const uploaded = await uploadImport(libraryId, selected, setProgress)
      setResult(uploaded)
    } catch (err) {
      setUploadError(errorMessage(err))
    } finally {
      setUploading(false)
    }
  }

  function handleFileInput(e: React.ChangeEvent<HTMLInputElement>) {
    if (e.target.files) void startUpload(filesFromFileList(e.target.files))
    e.target.value = ''
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragOver(false)
    void filesFromDataTransfer(e.dataTransfer.items).then(startUpload)
  }

  const totalBytes = files.reduce((sum, f) => sum + f.file.size, 0)
  const loadedBytes = progress?.loadedBytes ?? 0

  return (
    <div>
      <div
        className={dragOver ? 'upload-drop drag-over' : 'upload-drop'}
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
      >
        <div className="glyph">⬆</div>
        <p>Drag a folder here, or choose one from your device.</p>
        <button type="button" className="btn-ghost" disabled={uploading} onClick={() => inputRef.current?.click()}>
          Choose folder…
        </button>
        <input
          ref={inputRef}
          type="file"
          hidden
          multiple
          // @ts-expect-error webkitdirectory has no TS attribute typing but is
          // universally supported for a folder-picking file input.
          webkitdirectory=""
          onChange={handleFileInput}
        />
      </div>

      {files.length > 0 && (
        <div className="upload-file-list">
          {files.map((f, i) => {
            const { start, end } = offsets[i]
            const size = end - start
            let statusLabel: string
            let pct: number
            if (result || loadedBytes >= end) {
              statusLabel = '✓'
              pct = 100
            } else if (loadedBytes > start) {
              pct = size > 0 ? Math.round(((loadedBytes - start) / size) * 100) : 100
              statusLabel = `${pct}%`
            } else {
              pct = 0
              statusLabel = 'queued'
            }
            return (
              <div className="upload-file-row" key={f.relativePath}>
                <span className="fname">{f.relativePath}</span>
                <span className="fsize">{formatBytes(f.file.size)}</span>
                <div className="fbar">
                  <div style={{ width: `${pct}%` }} />
                </div>
                <span className={statusLabel === '✓' ? 'fstatus ok' : 'fstatus'}>{statusLabel}</span>
              </div>
            )
          })}
        </div>
      )}

      {files.length > 0 && (
        <div className="upload-summary">
          <span>
            {files.filter((_, i) => result || loadedBytes >= offsets[i].end).length} of {files.length} files uploaded
            ({formatBytes(loadedBytes)} / {formatBytes(totalBytes)})
          </span>
        </div>
      )}

      {uploadError && <p className="picker-status error">{uploadError}</p>}

      <div className="review-foot no-border">
        <button type="button" className="btn-ghost" onClick={onBack}>
          Back
        </button>
        <button type="button" className="btn-primary" disabled={!result} onClick={() => result && onUploaded(result)}>
          Next: review plan →
        </button>
      </div>
    </div>
  )
}
