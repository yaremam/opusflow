import { useState } from 'react'
import type { ArtStatus } from '../api/library'
import ArtActions from './ArtActions'
import ArtTile from './ArtTile'
import MergeModal, { type MergeCandidate } from './MergeModal'
import RemoveModal from './RemoveModal'

interface RemoveState {
  removing: boolean
  submitting: boolean
  error: string | null
  start: () => void
  cancel: () => void
  confirm: (deleteFiles: boolean) => void
}

interface MergeConfig {
  sourceSub: string
  effects: string[]
  search: (query: string) => Promise<MergeCandidate[]>
  merge: (intoId: number) => Promise<unknown>
  onMerged: (intoId: number) => void
}

interface EntityDetailHeaderProps {
  kind: 'artist' | 'album'
  displayName: string
  bannerSrc: string
  avatarSrc: string
  byLine?: React.ReactNode
  facts: React.ReactNode
  thumbUrl: string
  artStatus: ArtStatus
  onRetryArt: () => Promise<void>
  remove: RemoveState
  merge: MergeConfig
}

// EntityDetailHeader is the banner/avatar hero, art-retry action, and
// merge/remove flow shared verbatim by ArtistDetailPage and AlbumDetailPage
// (TDR 016's banner/avatar layout plus TDR 018's merge button, both of
// which used to be hand-duplicated inline in each page). Those two features
// landed independent edits to the same unseamed block eleven commits
// apart; a git merge auto-resolved them into invalid JSX that broke the
// build (see a7bbb68) — this component is the seam that should have existed
// before that collision, not just a cleanup after it.
export default function EntityDetailHeader({
  kind,
  displayName,
  bannerSrc,
  avatarSrc,
  byLine,
  facts,
  thumbUrl,
  artStatus,
  onRetryArt,
  remove,
  merge,
}: EntityDetailHeaderProps) {
  const [merging, setMerging] = useState(false)
  const kindLabel = kind === 'artist' ? 'Artist' : 'Album'

  return (
    <>
      <div className="detail-banner-wrap">
        <ArtTile src={bannerSrc} alt="" className="detail-banner-img" kind={kind} artStatus={artStatus} />
        <div className="detail-header-body">
          <ArtTile src={avatarSrc} alt="" className="detail-avatar" kind={kind} artStatus={artStatus} />
          <div className="detail-meta">
            <div className="kind">{kindLabel}</div>
            <h1>{displayName}</h1>
            {byLine}
            <div className="facts">{facts}</div>
            <ArtActions thumbUrl={thumbUrl} artStatus={artStatus} onRetry={onRetryArt} />
            <div className="detail-secondary-actions">
              <button type="button" className="btn-ghost" onClick={() => setMerging(true)}>
                ⇄ Merge into…
              </button>
              <button type="button" className="btn-ghost detail-remove" onClick={remove.start}>
                Remove {kind}…
              </button>
            </div>
          </div>
        </div>
      </div>

      {remove.removing && (
        <RemoveModal
          name={displayName}
          submitting={remove.submitting}
          submitError={remove.error}
          onDeleteFiles={() => remove.confirm(true)}
          onKeepFiles={() => remove.confirm(false)}
          onCancel={remove.cancel}
        />
      )}

      {merging && (
        <MergeModal
          label={kind}
          sourceName={displayName}
          sourceSub={merge.sourceSub}
          effects={merge.effects}
          search={merge.search}
          merge={merge.merge}
          onClose={() => setMerging(false)}
          onMerged={(intoId) => {
            setMerging(false)
            merge.onMerged(intoId)
          }}
        />
      )}
    </>
  )
}
