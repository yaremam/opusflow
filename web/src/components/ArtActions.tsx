import { useState } from 'react'
import { errorMessage, hasArtProblem, type ArtStatus } from '../api/library'
import './ArtActions.css'

interface ArtActionsProps {
  thumbUrl: string
  artStatus: ArtStatus
  onRetry: () => Promise<void>
}

// ArtActions is the "Retry lookup" action (TDR 007) shown above an artist/
// album's artwork gallery — a bordered button when there's no primary
// image to fall back on (a real problem), a quiet ghost button once one is
// already showing (a manual re-trigger, not a fix). Uploading/adding a new
// image lives in ArtworkGallery instead (TDR 014) — that's a gallery
// action, not a "fix this problem" one. onRetry owns the actual network
// call (and the poll loop); this component only owns the button/
// status-pill chrome and busy/error state around it.
export default function ArtActions({ thumbUrl, artStatus, onRetry }: ArtActionsProps) {
  const [retrying, setRetrying] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const problem = hasArtProblem(artStatus, thumbUrl)

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
        <button type="button" className={btnClass} disabled={retrying} onClick={handleRetry}>
          {retrying ? 'Retrying…' : '↻ Retry lookup'}
        </button>
      </div>
      {error && <p className="art-actions-error">{error}</p>}
    </div>
  )
}
