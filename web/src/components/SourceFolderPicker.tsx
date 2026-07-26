import { useEffect, useState } from 'react'
import { browse, createFolder, errorMessage, getConfig, type Entry } from '../api/library'
import './SourceFolderPicker.css'

interface NameFieldProps {
  label: string
  value: string
  onChange: (value: string) => void
}

interface SourceFolderPickerProps {
  title: string
  description: string
  confirmLabel: string
  confirmingLabel: string
  cancelLabel?: string
  nameField?: NameFieldProps
  onCancel: () => void
  onConfirm: (path: string) => void
  submitting: boolean
  submitError: string | null
}

interface BreadcrumbSegment {
  label: string
  path: string
}

// toBreadcrumb splits currentPath into clickable segments below root,
// pairing each segment's display label with the actual path clicking it
// should navigate to — so a click handler never has to reconstruct a path
// from labels. root itself is always the first segment: with DATA_DIR
// configured that's the browse root (e.g. "/data"), not the filesystem
// root, since there's nothing above it to browse into anyway (TDR 006,
// amended — see ARCHITECTURE.md's DATA_DIR entry).
function toBreadcrumb(currentPath: string, root: string): BreadcrumbSegment[] {
  const rootBase = root === '/' ? '' : root.replace(/\/$/, '')
  const remainder = currentPath.slice(rootBase.length).split('/').filter(Boolean)
  const segments: BreadcrumbSegment[] = [{ label: root, path: root }]
  let path = rootBase
  for (const part of remainder) {
    path = `${path}/${part}`
    segments.push({ label: part, path })
  }
  return segments
}

// SourceFolderPicker browses the container's filesystem — confined to
// DATA_DIR when the backend has one configured, unrestricted from "/"
// otherwise (TDR 006, amended) — so the reviewer can pick a folder to
// import from or a new library's root. Generalized
// (title/description/labels as props, plus an optional name field) so the
// same breadcrumb browser serves both the standalone "browse a server
// folder" step and the create-a-library form, without duplicating the
// browse logic.
export default function SourceFolderPicker({
  title,
  description,
  confirmLabel,
  confirmingLabel,
  cancelLabel = 'Cancel',
  nameField,
  onCancel,
  onConfirm,
  submitting,
  submitError,
}: SourceFolderPickerProps) {
  // root is the configured DATA_DIR (or "/" when unrestricted) — null
  // until that's known, which gates the browse effect below so it never
  // fires against the wrong starting path (e.g. "/" when DATA_DIR is set).
  const [root, setRoot] = useState<string | null>(null)
  const [currentPath, setCurrentPath] = useState('/')
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(true)
  const [browseError, setBrowseError] = useState<string | null>(null)

  const [creatingFolder, setCreatingFolder] = useState(false)
  const [newFolderName, setNewFolderName] = useState('')
  const [newFolderSubmitting, setNewFolderSubmitting] = useState(false)
  const [newFolderError, setNewFolderError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    getConfig()
      .then((cfg) => {
        if (cancelled) return
        const effectiveRoot = cfg.dataDir || '/'
        setRoot(effectiveRoot)
        setCurrentPath(effectiveRoot)
      })
      .catch(() => {
        if (!cancelled) setRoot('/')
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (root === null) return // still resolving the starting path above
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
  }, [currentPath, root])

  async function handleCreateFolderConfirm() {
    setNewFolderSubmitting(true)
    setNewFolderError(null)
    try {
      const entry = await createFolder(currentPath, newFolderName.trim())
      setCreatingFolder(false)
      setNewFolderName('')
      setCurrentPath(entry.path)
    } catch (err) {
      setNewFolderError(errorMessage(err))
    } finally {
      setNewFolderSubmitting(false)
    }
  }

  const segments = toBreadcrumb(currentPath, root ?? '/')
  const canConfirm = Boolean(currentPath) && (!nameField || nameField.value.trim() !== '')

  return (
    <div className="picker-scrim" onClick={(e) => e.target === e.currentTarget && onCancel()}>
      <div className="picker-panel" role="dialog" aria-modal="true" aria-label={title}>
        <div className="picker-head">
          <h2>{title}</h2>
          <p>{description}</p>
        </div>

        {nameField && (
          <div className="name-field">
            <label htmlFor="library-name">{nameField.label}</label>
            <input
              id="library-name"
              type="text"
              value={nameField.value}
              onChange={(e) => nameField.onChange(e.target.value)}
            />
          </div>
        )}

        <div className="picker-breadcrumb">
          <span className="breadcrumb-trail">
            {segments.map((seg, i) => (
              <span key={seg.path}>
                {i > 0 && <span className="sep">/</span>}
                <button type="button" className="crumb" onClick={() => setCurrentPath(seg.path)}>
                  {seg.label}
                </button>
              </span>
            ))}
          </span>
          {nameField && !creatingFolder && (
            <button
              type="button"
              className="new-folder-trigger"
              onClick={() => {
                setCreatingFolder(true)
                setNewFolderError(null)
              }}
            >
              ﹢ New folder
            </button>
          )}
        </div>

        {nameField && creatingFolder && (
          <div className="new-folder-row">
            <input
              type="text"
              autoFocus
              placeholder="Folder name"
              value={newFolderName}
              onChange={(e) => setNewFolderName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && newFolderName.trim() !== '') handleCreateFolderConfirm()
                if (e.key === 'Escape') {
                  setCreatingFolder(false)
                  setNewFolderName('')
                }
              }}
            />
            <button
              type="button"
              className="btn-ghost"
              onClick={() => {
                setCreatingFolder(false)
                setNewFolderName('')
                setNewFolderError(null)
              }}
            >
              Cancel
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={newFolderSubmitting || newFolderName.trim() === ''}
              onClick={handleCreateFolderConfirm}
            >
              {newFolderSubmitting ? 'Creating…' : 'Create'}
            </button>
            {newFolderError && <p className="picker-status error">{newFolderError}</p>}
          </div>
        )}

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
              {cancelLabel}
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={submitting || !canConfirm}
              onClick={() => onConfirm(currentPath)}
            >
              {submitting ? confirmingLabel : confirmLabel}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
