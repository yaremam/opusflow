import { rootLabel, type LibraryDirectory } from '../api/library'
import './DirectoryCard.css'

interface DirectoryCardProps {
  directory: LibraryDirectory
  onRemove: () => void
  removing: boolean
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

const STATUS_LABEL: Record<LibraryDirectory['status'], string> = {
  scanning: 'Scanning',
  complete: 'Complete',
  failed: 'Failed',
}

export default function DirectoryCard({ directory, onRemove, removing }: DirectoryCardProps) {
  const { root, path, status, filesProcessed, filesTotal, trackCount, fileErrors, error, createdAt } = directory
  const progressPct = filesTotal > 0 ? Math.min(100, (filesProcessed / filesTotal) * 100) : 0

  return (
    <div className="library-card">
      <div className="card-top">
        <div className="path-row">
          <span className="root-badge" title={root}>
            {rootLabel(root)}
          </span>
          <span className="card-path">{path}</span>
        </div>
        <div className="card-actions">
          <span className={`pill ${status}`}>
            {status === 'scanning' && <span className="pulse" />}
            {STATUS_LABEL[status]}
          </span>
          <button
            type="button"
            className="btn-icon"
            title={`Remove ${path}`}
            aria-label={`Remove ${path}`}
            disabled={removing}
            onClick={onRemove}
          >
            ✕
          </button>
        </div>
      </div>

      {status === 'scanning' ? (
        <>
          <div className="progress-track">
            <div className="progress-fill" style={{ width: `${progressPct}%` }} />
          </div>
          <div className="meta-row">
            <span className="count">
              {filesProcessed} of ~{filesTotal} files scanned
            </span>
          </div>
        </>
      ) : (
        <div className="meta-row">
          <span className="count">{trackCount} tracks imported</span>
          <span>Added {formatDate(createdAt)}</span>
        </div>
      )}

      {status === 'failed' && error && <div className="error-note">{error}</div>}

      {fileErrors.length > 0 && (
        <div className="error-note">
          {fileErrors.length} file{fileErrors.length === 1 ? '' : 's'} couldn't be read.
        </div>
      )}
    </div>
  )
}
