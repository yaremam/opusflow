import { useEffect, useState } from 'react'
import { browse, errorMessage, type Entry } from '../api/library'
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

// toBreadcrumb splits currentPath into clickable segments, pairing each
// segment's display label with the actual path clicking it should navigate
// to — so a click handler never has to reconstruct a path from labels. "/"
// itself is always the first segment, since there's no configured root to
// start from anymore (TDR 006) — browsing starts at the filesystem root.
function toBreadcrumb(currentPath: string): BreadcrumbSegment[] {
  const parts = currentPath.split('/').filter(Boolean)
  const segments: BreadcrumbSegment[] = [{ label: '/', path: '/' }]
  let path = ''
  for (const part of parts) {
    path = `${path}/${part}`
    segments.push({ label: part, path })
  }
  return segments
}

// SourceFolderPicker browses the container's filesystem, unrestricted from
// "/" (TDR 006 — there's no configured allowlist anymore), so the reviewer
// can pick a folder to import from or a new library's root. Generalized
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
  const [currentPath, setCurrentPath] = useState('/')
  const [entries, setEntries] = useState<Entry[]>([])
  const [loading, setLoading] = useState(false)
  const [browseError, setBrowseError] = useState<string | null>(null)

  useEffect(() => {
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

  const segments = toBreadcrumb(currentPath)
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
