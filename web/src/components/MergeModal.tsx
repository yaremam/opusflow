import { useEffect, useState } from 'react'
import { errorMessage } from '../api/library'
import './MergeModal.css'

export interface MergeCandidate {
  id: number
  name: string
  sub: string
}

interface MergeModalProps {
  label: string // "artist" | "album" — used in copy/button text
  sourceName: string
  sourceSub: string
  // effects are the merge-specific bullet points (what moves) — the
  // caller knows exactly what kind of entity this is, MergeModal doesn't;
  // the generic "remove the source, this can't be undone" line is added
  // by this component itself.
  effects: string[]
  search: (query: string) => Promise<MergeCandidate[]>
  merge: (intoId: number) => Promise<void>
  onClose: () => void
  onMerged: (intoId: number) => void
}

function initials(name: string): string {
  return (
    name
      .split(' ')
      .filter(Boolean)
      .map((w) => w[0])
      .join('')
      .slice(0, 2)
      .toUpperCase() || '?'
  )
}

// MergeModal is issue #31's "merge into..." flow: search-and-pick the
// surviving entity (step 1), then a summary of exactly what will move and
// an explicit can't-undo warning before the destructive action (step 2).
// Reuses the app's existing MetadataLookupModal search/step visual
// vocabulary (mdl-* classes) and RemoveModal's confirm-before-destroying
// stance, rather than inventing new patterns for what's structurally the
// same kind of decision.
export default function MergeModal({ label, sourceName, sourceSub, effects, search, merge, onClose, onMerged }: MergeModalProps) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<MergeCandidate[]>([])
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState<string | null>(null)
  const [target, setTarget] = useState<MergeCandidate | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setSearching(true)
    setSearchError(null)
    search(query)
      .then((r) => {
        if (!cancelled) setResults(r)
      })
      .catch((err: unknown) => {
        if (!cancelled) setSearchError(errorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setSearching(false)
      })
    return () => {
      cancelled = true
    }
  }, [query, search])

  async function handleConfirm() {
    if (!target) return
    setSubmitting(true)
    setSubmitError(null)
    try {
      await merge(target.id)
      onMerged(target.id)
    } catch (err) {
      setSubmitError(errorMessage(err))
      setSubmitting(false)
    }
  }

  return (
    <div className="mdl-scrim" onClick={(e) => e.target === e.currentTarget && !submitting && onClose()}>
      <div className="mdl-panel" role="dialog" aria-modal="true" aria-label={`Merge ${sourceName}`}>
        <div className="mdl-head">
          <div className="mdl-head-top">
            <div>
              <p className="mdl-eyebrow">Merge {label}</p>
              <h2>{target ? 'Confirm merge' : `Merge "${sourceName}" into which ${label}?`}</h2>
            </div>
            <button type="button" className="mdl-close" onClick={onClose} disabled={submitting} aria-label="Close">
              ✕
            </button>
          </div>
          <div className="mdl-steps">
            <div className={`mdl-step ${target ? 'done' : 'current'}`}>
              <span className="mdl-dot">1</span> Choose target
            </div>
            <div className={`mdl-step ${target ? 'current' : ''}`}>
              <span className="mdl-dot">2</span> Confirm
            </div>
          </div>
        </div>

        {!target && (
          <div className="mdl-body">
            <input
              className="mdl-search-input"
              placeholder={`Search your ${label}s…`}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              autoFocus
            />
            <p className="mdl-hint">
              Pick the {label} to keep — "{sourceName}" and everything on it will move onto whichever one you choose here, then "
              {sourceName}" is removed.
            </p>
            {searchError && <p className="mdl-error">{searchError}</p>}
            {!searching && !searchError && results.length === 0 && <p className="mdl-empty">No matching {label}s found.</p>}
            <div className="mdl-result-list">
              {results.map((r) => (
                <button type="button" key={r.id} className="mdl-result-card" onClick={() => setTarget(r)}>
                  <span className="mdl-result-avatar">{initials(r.name)}</span>
                  <span className="mdl-result-main">
                    <span className="mdl-result-title">{r.name}</span>
                    <span className="mdl-result-sub">{r.sub}</span>
                  </span>
                  <span className="mdl-result-arrow">›</span>
                </button>
              ))}
            </div>
          </div>
        )}

        {target && (
          <div className="mdl-body">
            <div className="merge-summary">
              <div className="merge-card loser">
                <div className="role">Removed</div>
                <div className="avatar">{initials(sourceName)}</div>
                <div className="name">{sourceName}</div>
                <div className="meta">{sourceSub}</div>
              </div>
              <div className="merge-arrow">→</div>
              <div className="merge-card winner">
                <div className="role">Kept</div>
                <div className="avatar">{initials(target.name)}</div>
                <div className="name">{target.name}</div>
                <div className="meta">{target.sub}</div>
              </div>
            </div>
            <div className="merge-effects">
              This will:
              <ul>
                {effects.map((e, i) => (
                  <li key={i}>{e}</li>
                ))}
                <li>Remove "{sourceName}" once everything has moved.</li>
              </ul>
              <div className="warn-line">This can't be undone.</div>
            </div>
            {submitError && <p className="mdl-error">{submitError}</p>}
          </div>
        )}

        <div className="mdl-foot">
          {target && (
            <button type="button" className="btn-ghost" disabled={submitting} onClick={() => setTarget(null)}>
              ‹ Back
            </button>
          )}
          {!target && (
            <button type="button" className="btn-ghost" onClick={onClose}>
              Cancel
            </button>
          )}
          {target && (
            <button type="button" className="btn-bad" disabled={submitting} onClick={handleConfirm}>
              {submitting ? 'Merging…' : `Merge ${label}s`}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
