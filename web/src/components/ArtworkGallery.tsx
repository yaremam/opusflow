import { useEffect, useRef, useState } from 'react'
import { errorMessage } from '../api/library'
import RemoveModal from './RemoveModal'
import './ArtworkGallery.css'

// GalleryImage is the common shape ArtistPhoto and AlbumCover both satisfy
// — ArtworkGallery renders either without caring which (TDR 014). isBanner
// is TDR 016's independent "used as the detail page header's banner" flag.
export interface GalleryImage {
  id: number
  thumbUrl: string
  fullUrl: string
  isPrimary: boolean
  isBanner: boolean
  source: string
  pictureType?: string
}

interface ArtworkGalleryProps {
  images: GalleryImage[]
  label: string // "photo" or "cover" — used in button/modal text
  onUpload: (file: File) => Promise<void>
  onSetPrimary: (id: number) => Promise<void>
  onSetBanner: (id: number) => Promise<void>
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
// pages (TDR 014, redesigned TDR 016): one image shown at a time in a
// small fixed-size viewer — paged by clicking its left/right half (or the
// arrow keys), never a wrapping grid or thumbnail strip, so the gallery's
// footprint stays constant regardless of how many images exist. An
// "expand" action opens the active image full-screen for inspecting
// detail (e.g. reading a booklet scan) instead of zooming in place.
export default function ArtworkGallery({ images, label, onUpload, onSetPrimary, onSetBanner, onDelete }: ArtworkGalleryProps) {
  const [current, setCurrent] = useState(0)
  const [uploading, setUploading] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [pendingRemoveId, setPendingRemoveId] = useState<number | null>(null)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const activeIndex = images.length === 0 ? 0 : Math.min(current, images.length - 1)
  const active = images[activeIndex] as GalleryImage | undefined

  function goPrev() {
    setCurrent((activeIndex - 1 + images.length) % images.length)
  }
  function goNext() {
    setCurrent((activeIndex + 1) % images.length)
  }

  // Arrow-key paging (AC-5) — skipped while focus is in a text field
  // elsewhere on the page, so this doesn't hijack cursor movement while
  // typing (e.g. a search box), and always active while the full-screen
  // view is open since that's a modal context.
  useEffect(() => {
    if (images.length < 2) return
    function handleKeyDown(e: KeyboardEvent) {
      const typing = document.activeElement instanceof HTMLInputElement || document.activeElement instanceof HTMLTextAreaElement
      if (typing && !expanded) return
      if (e.key === 'ArrowLeft') goPrev()
      if (e.key === 'ArrowRight') goNext()
      if (e.key === 'Escape' && expanded) setExpanded(false)
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [images.length, activeIndex, expanded])

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

  async function handleSetBanner(id: number) {
    setBusyId(id)
    setActionError(null)
    try {
      await onSetBanner(id)
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
        {images.length > 0 && (
          <span className="gallery-count">
            {activeIndex + 1} of {images.length}
          </span>
        )}
      </div>
      {(uploadError || actionError) && <p className="gallery-error">{uploadError || actionError}</p>}

      {active && (
        <div className="gallery-viewer">
          <button type="button" className="gallery-nav" onClick={goPrev} disabled={images.length < 2} aria-label={`Previous ${label}`}>
            ‹
          </button>
          <div className="gallery-stage">
            <div className="gallery-badges">
              {active.isPrimary && <span className="gallery-badge primary">★ Primary</span>}
              {active.isBanner && <span className="gallery-badge banner">⬒ Banner</span>}
            </div>
            <img src={active.thumbUrl} alt="" className="gallery-stage-img" />
            {images.length > 1 && (
              <>
                <button type="button" className="gallery-page-zone left" onClick={goPrev} aria-label={`Previous ${label}`}>
                  <span>‹</span>
                </button>
                <button type="button" className="gallery-page-zone right" onClick={goNext} aria-label={`Next ${label}`}>
                  <span>›</span>
                </button>
              </>
            )}
            <button type="button" className="gallery-expand" onClick={() => setExpanded(true)} title="View full size" aria-label="View full size">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M9 4H4v5M15 4h5v5M9 20H4v-5M15 20h5v-5" />
              </svg>
            </button>
          </div>
          <button type="button" className="gallery-nav" onClick={goNext} disabled={images.length < 2} aria-label={`Next ${label}`}>
            ›
          </button>
        </div>
      )}

      <div className="gallery-below">
        {active && (
          <div className="gallery-caption">
            {active.pictureType || sourceLabel(active.source)}
            {active.pictureType && <span className="gallery-source"> · {sourceLabel(active.source)}</span>}
          </div>
        )}
        <div className="gallery-actions">
          {active && !active.isPrimary && (
            <button type="button" disabled={busyId === active.id} onClick={() => handleSetPrimary(active.id)}>
              ☆ Primary
            </button>
          )}
          {active && !active.isBanner && (
            <button type="button" disabled={busyId === active.id} onClick={() => handleSetBanner(active.id)}>
              ⬒ Banner
            </button>
          )}
          {active && (
            <button type="button" className="remove" onClick={() => setPendingRemoveId(active.id)}>
              ✕ Remove
            </button>
          )}
          <button type="button" className="add" disabled={uploading} onClick={() => fileInputRef.current?.click()}>
            {uploading ? 'Uploading…' : `+ Add ${label}`}
          </button>
        </div>
        <input ref={fileInputRef} type="file" accept="image/*" hidden onChange={handleFileChosen} />
      </div>

      {expanded && active && (
        <div className="gallery-fullview" onClick={(e) => e.target === e.currentTarget && setExpanded(false)}>
          <button type="button" className="gallery-fv-close" onClick={() => setExpanded(false)} aria-label="Close">
            ✕
          </button>
          <div className="gallery-fv-stage">
            <img src={active.fullUrl || active.thumbUrl} alt="" className="gallery-fv-img" />
            {images.length > 1 && (
              <>
                <button type="button" className="gallery-page-zone left" onClick={goPrev} aria-label={`Previous ${label}`}>
                  <span>‹</span>
                </button>
                <button type="button" className="gallery-page-zone right" onClick={goNext} aria-label={`Next ${label}`}>
                  <span>›</span>
                </button>
              </>
            )}
          </div>
          <div className="gallery-fv-caption">
            {active.pictureType || sourceLabel(active.source)}
            {active.pictureType && <> · {sourceLabel(active.source)}</>} · {activeIndex + 1} of {images.length}
          </div>
        </div>
      )}

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
