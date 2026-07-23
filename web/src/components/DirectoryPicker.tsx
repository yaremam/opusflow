import { useEffect, useState } from 'react'
import { browse, errorMessage, rootLabel, type Entry, type RootInfo } from '../api/library'
import './DirectoryPicker.css'

interface DirectoryPickerProps {
  roots: RootInfo[]
  onCancel: () => void
  onConfirm: (path: string) => void
  submitting: boolean
  submitError: string | null
}

interface BreadcrumbSegment {
  label: string
  path: string
}

// toBreadcrumb walks currentPath from root, pairing each segment's display
// label with the actual path clicking it should navigate to — so a click
// handler never has to reconstruct a path from labels (which would need to
// stay in sync with how labels are computed here); it just uses the path
// that's already sitting right next to the label it rendered.
function toBreadcrumb(root: string, currentPath: string): BreadcrumbSegment[] {
  const relative = currentPath.slice(root.length).replace(/^\/+/, '')
  const parts = relative ? relative.split('/').filter(Boolean) : []

  const segments: BreadcrumbSegment[] = [{ label: rootLabel(root), path: root }]
  let path = root
  for (const part of parts) {
    path = `${path}/${part}`
    segments.push({ label: part, path })
  }
  return segments
}

export default function DirectoryPicker({
  roots,
  onCancel,
  onConfirm,
  submitting,
  submitError,
}: DirectoryPickerProps) {
  const [activeRoot, setActiveRoot] = useState(roots[0]?.path ?? '')
  const [currentPath, setCurrentPath] = useState(activeRoot)
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(false)
  const [browseError, setBrowseError] = useState<string | null>(null)

  // roots often arrives after this component has already mounted (the
  // picker opens as soon as the user clicks "Add directory," regardless of
  // whether LibraryPage's root fetch has resolved yet) — the useState
  // initializers above only run once, at mount, so back-fill activeRoot
  // once real roots show up if nothing has selected one yet.
  useEffect(() => {
    if (activeRoot || roots.length === 0) return
    setActiveRoot(roots[0].path)
    setCurrentPath(roots[0].path)
  }, [roots, activeRoot])

  function selectRoot(path: string) {
    setActiveRoot(path)
    setCurrentPath(path)
  }

  useEffect(() => {
    if (!currentPath) return
    let cancelled = false
    setLoading(true)
    setBrowseError(null)
    browse(currentPath)
      .then((result) => {
        if (!cancelled) setEntries(result)
      })
      .catch((err: unknown) => {
        if (!cancelled) setBrowseError(errorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [currentPath])

  const segments = toBreadcrumb(activeRoot, currentPath)

  return (
    <div className="picker-scrim" onClick={(e) => e.target === e.currentTarget && onCancel()}>
      <div className="picker-panel" role="dialog" aria-modal="true" aria-label="Add a directory">
        <div className="picker-head">
          <h2>Add a directory</h2>
          {roots.length > 1 && (
            <div className="picker-roots">
              {roots.map((r) => (
                <button
                  key={r.path}
                  type="button"
                  className={r.path === activeRoot ? 'root-tab active' : 'root-tab'}
                  onClick={() => selectRoot(r.path)}
                >
                  {r.path}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="picker-breadcrumb">
          {segments.map((seg, i) => (
            <span key={seg.path}>
              {i > 0 && <span className="sep">/</span>}
              <button type="button" className="crumb" onClick={() => setCurrentPath(seg.path)}>
                {seg.label}
              </button>
            </span>
          ))}
        </div>

        <div className="picker-folder-list">
          {loading && <p className="picker-status">Loading…</p>}
          {browseError && <p className="picker-status error">{browseError}</p>}
          {!loading && !browseError && entries.length === 0 && (
            <p className="picker-status">No subfolders here.</p>
          )}
          {!loading &&
            !browseError &&
            entries.map((entry) => (
              <button
                key={entry.path}
                type="button"
                className="folder-row"
                onClick={() => setCurrentPath(entry.path)}
              >
                📁 {entry.name}
                <span className="chev">›</span>
              </button>
            ))}
        </div>

        <div className="picker-foot">
          <span className="selected-path">
            Selected: <b>{currentPath}</b>
          </span>
          <div className="foot-actions">
            {submitError && <span className="picker-status error">{submitError}</span>}
            <button type="button" className="btn-ghost" onClick={onCancel}>
              Cancel
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={submitting || !currentPath}
              onClick={() => onConfirm(currentPath)}
            >
              {submitting ? 'Adding…' : 'Add this folder'}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
