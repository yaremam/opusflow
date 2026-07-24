import { useCallback, useEffect, useRef, useState } from 'react'
import {
  addDirectory,
  errorMessage,
  listDirectories,
  listRoots,
  removeDirectory,
  type LibraryDirectory,
  type RootInfo,
} from '../api/library'
import DirectoryPicker from '../components/DirectoryPicker'
import DirectoryCard from '../components/DirectoryCard'
import './LibraryPage.css'

const POLL_INTERVAL_MS = 1500

export default function LibraryPage() {
  const [directories, setDirectories] = useState<LibraryDirectory[]>([])
  const [roots, setRoots] = useState<RootInfo[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [adding, setAdding] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)
  const [removingId, setRemovingId] = useState<number | null>(null)

  // Guards against out-of-order responses: refresh() is called both by the
  // poll timer and directly after user actions (add/remove), so two calls
  // can be in flight at once. Only the response to the most recently
  // issued call is allowed to update state, so an older one that resolves
  // late can't overwrite fresher data.
  const refreshId = useRef(0)

  const refresh = useCallback(async () => {
    const id = ++refreshId.current
    try {
      const dirs = await listDirectories()
      if (id !== refreshId.current) return
      setDirectories(dirs)
      setLoadError(null)
    } catch (err) {
      if (id !== refreshId.current) return
      setLoadError(errorMessage(err))
    }
  }, [])

  useEffect(() => {
    listRoots()
      .then(setRoots)
      .catch((err: unknown) => setLoadError(errorMessage(err)))
    refresh()
  }, [refresh])

  const hasActiveScan = directories.some((d) => d.status === 'scanning')

  useEffect(() => {
    if (!hasActiveScan) return
    const id = setInterval(refresh, POLL_INTERVAL_MS)
    return () => clearInterval(id)
  }, [hasActiveScan, refresh])

  async function handleConfirmAdd(path: string) {
    setAdding(true)
    setAddError(null)
    try {
      await addDirectory(path)
      setPickerOpen(false)
      await refresh()
    } catch (err) {
      setAddError(errorMessage(err))
    } finally {
      setAdding(false)
    }
  }

  async function handleRemove(id: number) {
    setRemovingId(id)
    try {
      await removeDirectory(id)
      await refresh()
    } catch (err) {
      setLoadError(errorMessage(err))
    } finally {
      setRemovingId(null)
    }
  }

  return (
    <div className="page-shell">
      <div className="library-topbar">
        <div>
          <p className="eyebrow">Media library</p>
          <h1>Library</h1>
          <p className="sub">Directories scanned into your shared household library.</p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setPickerOpen(true)}>
          ＋ Add directory
        </button>
      </div>

      {loadError && <p className="library-load-error">{loadError}</p>}

      {directories.length === 0 && !loadError ? (
        <div className="library-empty">No directories yet — add one to start building your library.</div>
      ) : (
        <div className="library-list">
          {directories.map((dir) => (
            <DirectoryCard
              key={dir.id}
              directory={dir}
              removing={removingId === dir.id}
              onRemove={() => handleRemove(dir.id)}
            />
          ))}
        </div>
      )}

      {pickerOpen && (
        <DirectoryPicker
          roots={roots}
          submitting={adding}
          submitError={addError}
          onCancel={() => {
            setPickerOpen(false)
            setAddError(null)
          }}
          onConfirm={handleConfirmAdd}
        />
      )}
    </div>
  )
}
