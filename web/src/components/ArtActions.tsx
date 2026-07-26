import { useRef, useState } from 'react'
import { errorMessage, hasArtProblem, type ArtStatus } from '../api/library'
import './ArtActions.css'

interface ArtActionsProps {
  thumbUrl: string
  artStatus: ArtStatus
  label: string // "photo" or "cover" — used in the upload button's text
  onRetry: () => Promise<void>
  onUpload: (file: File) => Promise<void>
}

// ArtActions is the "Retry lookup" / "Upload photo|cover" action pair (TDR
// 007), shared by ArtistDetailPage and AlbumDetailPage since both follow
// the identical always-available rule: bordered buttons when there's no
// image to fall back on (a real problem), quiet ghost buttons once art is
// already showing (a manual override, not a fix). onRetry/onUpload own the
// actual network calls (and, for retry, the poll loop) — this component
// only owns the button/status-pill chrome and busy/error state around them.
export default function ArtActions({ thumbUrl, artStatus, label, onRetry, onUpload }: ArtActionsProps) {
  const [retrying, setRetrying] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const problem = hasArtProblem(artStatus, thumbUrl)
  const busy = retrying || uploading

  async function handleRetry() {
    setRetrying(true)
    setError(null)
    try {
      await onRetry()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setRetrying(false)
    }
  }

  async function handleFileChosen(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    e.target.value = '' // allow re-picking the same file next time
    if (!file) return
    setUploading(true)
    setError(null)
    try {
      await onUpload(file)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setUploading(false)
    }
  }

  const btnClass = problem ? 'btn-bordered' : 'btn-ghost-art'

  return (
    <div className="art-actions-wrap">
      {problem && (
        <div className={`art-status-pill ${artStatus === 'failed' ? 'bad' : 'warn'}`}>
          <span className="dot" />
          {artStatus === 'failed' ? 'Artwork lookup failed' : 'No artwork found'}
        </div>
      )}
      <div className="art-actions">
        <button type="button" className={btnClass} disabled={busy} onClick={handleRetry}>
          {retrying ? 'Retrying…' : '↻ Retry lookup'}
        </button>
        <button
          type="button"
          className={btnClass}
          disabled={busy}
          onClick={() => fileInputRef.current?.click()}
        >
          {uploading ? 'Uploading…' : `⬆ ${problem ? 'Upload' : 'Replace'} ${label}`}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          accept="image/*"
          hidden
          onChange={handleFileChosen}
        />
      </div>
      {error && <p className="art-actions-error">{error}</p>}
    </div>
  )
}
