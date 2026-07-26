import './RemoveModal.css'

interface RemoveModalProps {
  name: string
  onDeleteFiles: () => void
  onKeepFiles: () => void
  onCancel: () => void
  submitting: boolean
  submitError: string | null
}

// RemoveModal is TDR 005 AC-13's keep-vs-delete-files prompt, shown every
// time an artist or album is removed — opusflow owns the copy it made, so
// unlike the old scan-in-place model there's no safe default: the reviewer
// picks explicitly, every time, rather than either choice being assumed.
export default function RemoveModal({ name, onDeleteFiles, onKeepFiles, onCancel, submitting, submitError }: RemoveModalProps) {
  return (
    <div className="modal-scrim" onClick={(e) => e.target === e.currentTarget && !submitting && onCancel()}>
      <div className="modal-panel" role="dialog" aria-modal="true" aria-label={`Remove ${name}`}>
        <h2>Remove {name}?</h2>
        <p>
          This removes it from your library. You can also delete opusflow's copy of these files from disk — your
          choice, every time.
        </p>
        {submitError && <p className="modal-error">{submitError}</p>}
        <div className="modal-actions">
          <button type="button" className="btn-bad" disabled={submitting} onClick={onDeleteFiles}>
            Delete files from disk
          </button>
          <button type="button" className="btn-ghost" disabled={submitting} onClick={onKeepFiles}>
            Keep files, just remove from library
          </button>
          <button type="button" className="btn-ghost modal-cancel" disabled={submitting} onClick={onCancel}>
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}
