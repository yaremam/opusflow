import { useState } from 'react'
import {
  artistReleaseGroups,
  errorMessage,
  releaseGroupReleases,
  releaseTracks,
  searchArtists,
  type ArtistMatch,
  type MetadataTrack,
  type ReleaseGroupMatch,
  type ReleaseMatch,
} from '../api/library'
import '../styles/modal.css'
import './MetadataLookupModal.css'

type Step = 'artist' | 'album' | 'release' | 'tracks'

export interface MetadataLookupApply {
  artist: string
  album: string
  year: number
  // One entry per matched track, in the album's existing row order —
  // ImportPage applies these onto the corresponding track indices.
  tracks: { trackIndex: number; title: string; trackNumber: number }[]
}

interface MetadataLookupModalProps {
  initialArtist: string
  // Display labels for the album's current files, in review-row order —
  // used only to preview the track match, matching TDR 012's "match by
  // row order" decision.
  files: string[]
  onApply: (result: MetadataLookupApply) => void
  onClose: () => void
}

const STEPS: { key: Step; label: string }[] = [
  { key: 'artist', label: 'Artist' },
  { key: 'album', label: 'Album' },
  { key: 'release', label: 'Release' },
  { key: 'tracks', label: 'Tracks' },
]

export default function MetadataLookupModal({ initialArtist, files, onApply, onClose }: MetadataLookupModalProps) {
  const [step, setStep] = useState<Step>('artist')

  const [artistQuery, setArtistQuery] = useState(initialArtist)
  const [artistResults, setArtistResults] = useState<ArtistMatch[] | null>(null)
  const [artistLoading, setArtistLoading] = useState(false)
  const [artistError, setArtistError] = useState<string | null>(null)
  const [pickedArtist, setPickedArtist] = useState<ArtistMatch | null>(null)

  const [albumResults, setAlbumResults] = useState<ReleaseGroupMatch[] | null>(null)
  const [albumLoading, setAlbumLoading] = useState(false)
  const [albumError, setAlbumError] = useState<string | null>(null)
  const [pickedAlbum, setPickedAlbum] = useState<ReleaseGroupMatch | null>(null)

  const [releaseResults, setReleaseResults] = useState<ReleaseMatch[] | null>(null)
  const [releaseLoading, setReleaseLoading] = useState(false)
  const [releaseError, setReleaseError] = useState<string | null>(null)
  const [pickedRelease, setPickedRelease] = useState<ReleaseMatch | null>(null)

  const [trackResults, setTrackResults] = useState<MetadataTrack[] | null>(null)
  const [trackLoading, setTrackLoading] = useState(false)
  const [trackError, setTrackError] = useState<string | null>(null)

  function runArtistSearch() {
    if (!artistQuery.trim()) return
    setArtistLoading(true)
    setArtistError(null)
    searchArtists(artistQuery.trim())
      .then(setArtistResults)
      .catch((err: unknown) => setArtistError(errorMessage(err)))
      .finally(() => setArtistLoading(false))
  }

  function pickArtist(artist: ArtistMatch) {
    setPickedArtist(artist)
    setStep('album')
    setAlbumResults(null)
    setAlbumError(null)
    setAlbumLoading(true)
    artistReleaseGroups(artist.mbid)
      .then(setAlbumResults)
      .catch((err: unknown) => setAlbumError(errorMessage(err)))
      .finally(() => setAlbumLoading(false))
  }

  function pickAlbum(album: ReleaseGroupMatch) {
    setPickedAlbum(album)
    setStep('release')
    setReleaseResults(null)
    setReleaseError(null)
    setReleaseLoading(true)
    releaseGroupReleases(album.mbid)
      .then(setReleaseResults)
      .catch((err: unknown) => setReleaseError(errorMessage(err)))
      .finally(() => setReleaseLoading(false))
  }

  function pickRelease(release: ReleaseMatch) {
    setPickedRelease(release)
    setStep('tracks')
    setTrackResults(null)
    setTrackError(null)
    setTrackLoading(true)
    releaseTracks(release.mbid)
      .then(setTrackResults)
      .catch((err: unknown) => setTrackError(errorMessage(err)))
      .finally(() => setTrackLoading(false))
  }

  function apply() {
    if (!pickedArtist || !pickedAlbum || !trackResults) return
    const matchedCount = Math.min(files.length, trackResults.length)
    const tracks = trackResults.slice(0, matchedCount).map((tr, i) => ({
      trackIndex: i,
      title: tr.title,
      trackNumber: i + 1,
    }))
    onApply({ artist: pickedArtist.name, album: pickedAlbum.title, year: pickedAlbum.firstReleaseYear ?? 0, tracks })
    onClose()
  }

  function backTo(target: Step) {
    setStep(target)
  }

  const stepIndex = STEPS.findIndex((s) => s.key === step)

  return (
    <div className="mdl-scrim" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="mdl-panel" role="dialog" aria-modal="true" aria-label="Look up metadata">
        <div className="mdl-head">
          <div className="mdl-head-top">
            <div>
              <p className="mdl-eyebrow">Look up metadata</p>
              <h2>
                {step === 'artist' && 'Search for the artist'}
                {step === 'album' && `${pickedArtist?.name} — pick the album`}
                {step === 'release' && `${pickedAlbum?.title} — pick the release`}
                {step === 'tracks' && 'Review the track match'}
              </h2>
            </div>
            <button type="button" className="mdl-close" aria-label="Close" onClick={onClose}>
              ✕
            </button>
          </div>
          <div className="mdl-steps">
            {STEPS.map((s, i) => (
              <div key={s.key} className={i < stepIndex ? 'mdl-step done' : i === stepIndex ? 'mdl-step current' : 'mdl-step'}>
                <span className="mdl-dot">{i < stepIndex ? '✓' : i + 1}</span> {s.label}
              </div>
            ))}
          </div>
        </div>

        <div className="mdl-body">
          {step === 'artist' && (
            <>
              <div className="mdl-field-row">
                <input
                  className="mdl-search-input"
                  value={artistQuery}
                  onChange={(e) => setArtistQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && runArtistSearch()}
                  placeholder="Artist name"
                />
                <button type="button" className="btn-primary" disabled={artistLoading} onClick={runArtistSearch}>
                  {artistLoading ? 'Searching…' : 'Search'}
                </button>
              </div>
              {artistError && <p className="mdl-error">{artistError}</p>}
              {artistResults && artistResults.length === 0 && !artistError && <p className="mdl-hint">No matches found.</p>}
              {artistResults && artistResults.length > 0 && (
                <div className="mdl-result-list">
                  {artistResults.map((a) => (
                    <button type="button" className="mdl-result-card" key={a.mbid} onClick={() => pickArtist(a)}>
                      <div className="mdl-result-avatar">♪</div>
                      <div className="mdl-result-main">
                        <div className="mdl-result-title">{a.name}</div>
                        {a.disambiguation && <div className="mdl-result-sub">{a.disambiguation}</div>}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </>
          )}

          {step === 'album' && (
            <>
              {albumLoading && <p className="mdl-hint">Loading albums…</p>}
              {albumError && <p className="mdl-error">{albumError}</p>}
              {albumResults && albumResults.length === 0 && !albumError && <p className="mdl-hint">No albums found for this artist.</p>}
              {albumResults && albumResults.length > 0 && (
                <div className="mdl-result-list">
                  {albumResults.map((g) => (
                    <button type="button" className="mdl-result-card" key={g.mbid} onClick={() => pickAlbum(g)}>
                      <div className="mdl-result-art">▦</div>
                      <div className="mdl-result-main">
                        <div className="mdl-result-title">{g.title}</div>
                      </div>
                      {g.firstReleaseYear ? <div className="mdl-result-meta">{g.firstReleaseYear}</div> : null}
                    </button>
                  ))}
                </div>
              )}
            </>
          )}

          {step === 'release' && (
            <>
              <p className="mdl-hint">Track listings live on a specific release, not the album itself — pick the edition matching what you have.</p>
              {releaseLoading && <p className="mdl-hint">Loading releases…</p>}
              {releaseError && <p className="mdl-error">{releaseError}</p>}
              {releaseResults && releaseResults.length === 0 && !releaseError && <p className="mdl-hint">No releases found for this album.</p>}
              {releaseResults && releaseResults.length > 0 && (
                <div className="mdl-result-list">
                  {releaseResults.map((r) => (
                    <button type="button" className="mdl-result-card" key={r.mbid} onClick={() => pickRelease(r)}>
                      <div className="mdl-result-art">▦</div>
                      <div className="mdl-result-main">
                        <div className="mdl-result-title">
                          {r.country || 'Unknown region'}
                          {r.date ? ` · ${r.date}` : ''}
                        </div>
                      </div>
                      <div className="mdl-result-meta">{r.trackCount} track{r.trackCount === 1 ? '' : 's'}</div>
                    </button>
                  ))}
                </div>
              )}
            </>
          )}

          {step === 'tracks' && (
            <>
              {pickedRelease && (
                <p className="mdl-hint">
                  {pickedRelease.country || 'Unknown region'}
                  {pickedRelease.date ? ` · ${pickedRelease.date}` : ''}
                </p>
              )}
              {trackLoading && <p className="mdl-hint">Loading track listing…</p>}
              {trackError && <p className="mdl-error">{trackError}</p>}
              {trackResults && (
                <>
                  {(() => {
                    const matchedCount = Math.min(files.length, trackResults.length)
                    const extra = trackResults.length - matchedCount
                    return (
                      <div className={extra > 0 ? 'mdl-match-summary warn' : 'mdl-match-summary'}>
                        {extra > 0
                          ? `⚠ ${matchedCount} of ${trackResults.length} tracks matched — ${extra} track${extra === 1 ? '' : 's'} from this release ${extra === 1 ? 'has' : 'have'} no file and will be left out.`
                          : `All ${matchedCount} tracks matched to files.`}
                      </div>
                    )
                  })()}
                  <div className="mdl-table-wrap">
                    <table className="mdl-match-table">
                      <thead>
                        <tr>
                          <th className="mdl-col-num">#</th>
                          <th>Your file</th>
                          <th className="mdl-col-arrow" />
                          <th>Matched track title</th>
                        </tr>
                      </thead>
                      <tbody>
                        {trackResults.map((tr, i) => {
                          const file = files[i]
                          return (
                            <tr key={i} className={file ? undefined : 'mdl-unmatched-row'}>
                              <td className="mdl-col-num">{i + 1}</td>
                              <td className="mdl-file-name">{file || '—'}</td>
                              <td className="mdl-col-arrow">{file ? '→' : ''}</td>
                              <td className="mdl-track-title">{tr.title}</td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>
                </>
              )}
            </>
          )}
        </div>

        <div className="mdl-foot">
          <div className="mdl-foot-left">
            {step === 'album' && (
              <button type="button" className="btn-ghost" onClick={() => backTo('artist')}>
                Back
              </button>
            )}
            {step === 'release' && (
              <button type="button" className="btn-ghost" onClick={() => backTo('album')}>
                Back
              </button>
            )}
            {step === 'tracks' && (
              <button type="button" className="btn-ghost" onClick={() => backTo('release')}>
                Back
              </button>
            )}
            {step === 'artist' && (
              <button type="button" className="btn-ghost" onClick={onClose}>
                Cancel
              </button>
            )}
          </div>
          {step === 'tracks' && trackResults && (
            <button type="button" className="btn-primary" onClick={apply}>
              Apply to album
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
