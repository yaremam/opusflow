import { useRef, useState } from 'react'
import { errorMessage } from '../api/library'
import RemoveModal from './RemoveModal'
import './ArtworkGallery.css'

// GalleryImage is the common shape ArtistPhoto and AlbumCover both satisfy
// — ArtworkGallery renders either without caring which (TDR 014).
export interface GalleryImage {
  id: number
  thumbUrl: string
  isPrimary: boolean
  source: string
  pictureType?: string
}

interface ArtworkGalleryProps {
  images: GalleryImage[]
  label: string // "photo" or "cover" — used in button/modal text
  onUpload: (file: File) => Promise<void>
  onSetPrimary: (id: number) => Promise<void>
  onDelete: (id: number, deleteFile: boolean) => Promise<void>
}

// sourceLabel turns a raw source string ("cover_art_archive", "upload", …)
// into the label a reviewer recognizes — the backend value is whatever's
// convenient to store and match on, not necessarily display-ready.
function sourceLabel(source: string): string {
  switch (source) {
    case 'cover_art_archive':
      return 'Cover Art Archive'
    case 'embedded':
      return 'Embedded tag'
    case 'enrichment':
      return 'Automatic lookup'
    case 'upload':
      return 'Upload'
    case 'legacy':
      return 'Existing image'
    default:
      return source
  }
}

// ArtworkGallery is the multi-image gallery on the Artist/Album detail
// pages (TDR 014): every image in the entity's gallery, each individually
// promotable to primary and removable (reusing the app's existing
// keep-vs-delete-file RemoveModal), plus an always-available "add" tile —
// uploading here always adds a new image, it never replaces one (AC-3).
export default function ArtworkGallery({ images, label, onUpload, onSetPrimary, onDelete }: ArtworkGalleryProps) {
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [pendingRemoveId, setPendingRemoveId] = useState<number | null>(null)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  async function handleFileChosen(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = ''
    if (!file) return
    setUploading(true)
    setUploadError(null)
    try {
      await onUpload(file)
    } catch (err) {
      setUploadError(errorMessage(err))
    } finally {
      setUploading(false)
    }
  }

  async function handleSetPrimary(id: number) {
    setBusyId(id)
    setActionError(null)
    try {
      await onSetPrimary(id)
    } catch (err) {
      setActionError(errorMessage(err))
    } finally {
      setBusyId(null)
    }
  }

  async function handleRemove(deleteFile: boolean) {
    if (pendingRemoveId == null) return
    setRemoveSubmitting(true)
    setRemoveError(null)
    try {
      await onDelete(pendingRemoveId, deleteFile)
      setPendingRemoveId(null)
    } catch (err) {
      setRemoveError(errorMessage(err))
    } finally {
      setRemoveSubmitting(false)
    }
  }

  return (
    <>
      <div className="section-head">
        <h2>Artwork</h2>
        <span className="gallery-count">
          {images.length} image{images.length === 1 ? '' : 's'}
        </span>
      </div>
      {(uploadError || actionError) && <p className="gallery-error">{uploadError || actionError}</p>}
      <div className="gallery-grid">
        {images.map((img) => (
          <div key={img.id} className={`gallery-tile${img.isPrimary ? ' is-primary' : ''}`}>
            <div className="thumb-wrap">
              <img src={img.thumbUrl} alt="" loading="lazy" />
              {img.isPrimary && <span className="primary-badge">★ Primary</span>}
            </div>
            <div className="caption">{img.pictureType || sourceLabel(img.source)}</div>
            <div className="source">{img.pictureType ? sourceLabel(img.source) : ' '}</div>
            <div className="tile-actions">
              {!img.isPrimary && (
                <button type="button" disabled={busyId === img.id} onClick={() => handleSetPrimary(img.id)}>
                  ☆ Set primary
                </button>
              )}
              <button type="button" className="remove" onClick={() => setPendingRemoveId(img.id)}>
                ✕ Remove
              </button>
            </div>
          </div>
        ))}
        <button
          type="button"
          className="gallery-add"
          disabled={uploading}
          onClick={() => fileInputRef.current?.click()}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
            <path d="M12 5v14M5 12h14" />
          </svg>
          {uploading ? 'Uploading…' : `Add ${label}`}
        </button>
        <input ref={fileInputRef} type="file" accept="image/*" hidden onChange={handleFileChosen} />
      </div>

      {pendingRemoveId != null && (
        <RemoveModal
          name={`this ${label}`}
          description="Keep opusflow's copy of the file on disk, or delete it too — your choice, every time."
          submitting={removeSubmitting}
          submitError={removeError}
          onDeleteFiles={() => handleRemove(true)}
          onKeepFiles={() => handleRemove(false)}
          onCancel={() => {
            setPendingRemoveId(null)
            setRemoveError(null)
          }}
        />
      )}
    </>
  )
}
