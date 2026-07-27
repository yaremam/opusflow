import { Fragment, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import {
  buildPlan,
  confirmImport,
  createLibrary,
  errorMessage,
  getImport,
  listImports,
  listLibraries,
  validatePlan,
  type Import,
  type Library,
  type Plan,
  type PlanAlbum,
  type PlanResponse,
  type ValidationError,
} from '../api/library'
import MetadataLookupModal, { type MetadataLookupApply } from '../components/MetadataLookupModal'
import SourceFolderPicker from '../components/SourceFolderPicker'
import UploadDropzone from '../components/UploadDropzone'
import './ImportPage.css'

type Step = 'list' | 'library' | 'createLibrary' | 'source' | 'browse' | 'upload' | 'review' | 'copying' | 'done'

const POLL_INTERVAL_MS = 1200
const HISTORY_EXPANDED_KEY = 'opusflow.importHistoryExpanded'
const HISTORY_PAGE_SIZE = 10

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function withAlbumField(plan: Plan, albumIndex: number, field: 'artist' | 'album' | 'year', value: string): Plan {
  return {
    albums: plan.albums.map((al, i): PlanAlbum => (i === albumIndex ? { ...al, [field]: field === 'year' ? Number(value) || 0 : value } : al)),
  }
}

function withTrackField(plan: Plan, albumIndex: number, trackIndex: number, field: 'title' | 'trackNumber', value: string): Plan {
  return {
    albums: plan.albums.map((al, ai) => {
      if (ai !== albumIndex) return al
      return {
        ...al,
        tracks: al.tracks.map((tr, ti) =>
          ti === trackIndex ? { ...tr, [field]: field === 'trackNumber' ? Number(value) || 0 : value } : tr,
        ),
      }
    }),
  }
}

function withOverwrite(plan: Plan, albumIndex: number, trackIndex: number, overwrite: boolean): Plan {
  return {
    albums: plan.albums.map((al, ai) => {
      if (ai !== albumIndex) return al
      return { ...al, tracks: al.tracks.map((tr, ti) => (ti === trackIndex ? { ...tr, overwrite } : tr)) }
    }),
  }
}

function withOverwriteAlbum(plan: Plan, albumIndex: number): Plan {
  return {
    albums: plan.albums.map((al, ai) => {
      if (ai !== albumIndex) return al
      return { ...al, tracks: al.tracks.map((tr) => (tr.conflict ? { ...tr, overwrite: true } : tr)) }
    }),
  }
}

// withMetadataLookup applies a picked MusicBrainz release (TDR 012) onto
// one album: artist/album/year unconditionally, and each matched track's
// title/trackNumber by index — tracks with no match (a count mismatch
// between the release and the album's files) are left untouched.
function withMetadataLookup(plan: Plan, albumIndex: number, result: MetadataLookupApply): Plan {
  const byTrackIndex = new Map(result.tracks.map((t) => [t.trackIndex, t]))
  return {
    albums: plan.albums.map((al, ai) => {
      if (ai !== albumIndex) return al
      return {
        ...al,
        artist: result.artist,
        album: result.album,
        year: result.year,
        tracks: al.tracks.map((tr, ti) => {
          const match = byTrackIndex.get(ti)
          return match ? { ...tr, title: match.title, trackNumber: match.trackNumber } : tr
        }),
      }
    }),
  }
}

function errorFor(errors: ValidationError[], albumIndex: number, trackIndex: number): ValidationError | undefined {
  return errors.find((e) => e.albumIndex === albumIndex && e.trackIndex === trackIndex)
}

function trackKey(albumIndex: number, trackIndex: number): string {
  return `${albumIndex}:${trackIndex}`
}

type Selection = 'all' | 'none' | 'mixed'

function isExcluded(albumIndex: number, trackIndex: number, excluded: Set<string>): boolean {
  return excluded.has(trackKey(albumIndex, trackIndex))
}

function albumSelection(al: PlanAlbum, albumIndex: number, excluded: Set<string>): Selection {
  if (al.tracks.length === 0) return 'all'
  const excludedCount = al.tracks.filter((_, ti) => excluded.has(trackKey(albumIndex, ti))).length
  if (excludedCount === 0) return 'all'
  if (excludedCount === al.tracks.length) return 'none'
  return 'mixed'
}

function includedTrackCount(plan: Plan, excluded: Set<string>): number {
  return plan.albums.reduce(
    (sum, al, ai) => sum + al.tracks.filter((_, ti) => !excluded.has(trackKey(ai, ti))).length,
    0,
  )
}

// buildIncludedPlan strips excluded tracks out of the payload sent to
// validatePlan/confirmImport — the backend never needs to know a track was
// excluded, it just never sees it (TDR 011). indexMaps[albumIndex][j] is
// the original trackIndex the j-th included track came from, needed to
// translate the response's plan/errors (indexed into the filtered arrays)
// back onto the full local plan's original indices.
function buildIncludedPlan(plan: Plan, excluded: Set<string>): { plan: Plan; indexMaps: number[][] } {
  const indexMaps: number[][] = []
  const albums = plan.albums.map((al, ai) => {
    const map: number[] = []
    const tracks = al.tracks.filter((_, ti) => {
      const keep = !excluded.has(trackKey(ai, ti))
      if (keep) map.push(ti)
      return keep
    })
    indexMaps.push(map)
    return { ...al, tracks }
  })
  return { plan: { albums }, indexMaps }
}

function remapErrors(errors: ValidationError[], indexMaps: number[][]): ValidationError[] {
  return errors.map((e) => ({ ...e, trackIndex: indexMaps[e.albumIndex]?.[e.trackIndex] ?? e.trackIndex }))
}

// mergeIncludedPlan folds validatePlan's corrected fields (computed only
// for the included subset, see buildIncludedPlan) back into the full local
// plan — excluded tracks keep whatever they already had, since the backend
// never touched them.
function mergeIncludedPlan(fullPlan: Plan, resultPlan: Plan, indexMaps: number[][]): Plan {
  return {
    albums: fullPlan.albums.map((al, ai): PlanAlbum => {
      const resultAlbum = resultPlan.albums[ai]
      const map = indexMaps[ai] ?? []
      return {
        ...resultAlbum,
        tracks: al.tracks.map((tr, ti) => {
          const pos = map.indexOf(ti)
          return pos === -1 ? tr : resultAlbum.tracks[pos]
        }),
      }
    }),
  }
}

export default function ImportPage() {
  const [step, setStep] = useState<Step>('list')

  const [imports, setImports] = useState<Import[]>([])
  const [listError, setListError] = useState<string | null>(null)
  const [historyExpanded, setHistoryExpanded] = useState(() => localStorage.getItem(HISTORY_EXPANDED_KEY) === 'true')
  const [historyVisibleCount, setHistoryVisibleCount] = useState(HISTORY_PAGE_SIZE)

  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryListError, setLibraryListError] = useState<string | null>(null)
  const [libraryId, setLibraryId] = useState<number | null>(null)
  const [createName, setCreateName] = useState('')
  const [createSubmitting, setCreateSubmitting] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const [sourceDescription, setSourceDescription] = useState('')
  const [plan, setPlan] = useState<Plan | null>(null)
  const [errors, setErrors] = useState<ValidationError[]>([])
  const [planLoading, setPlanLoading] = useState(false)
  const [planLoadError, setPlanLoadError] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [confirmError, setConfirmError] = useState<string | null>(null)
  const [bannerFlash, setBannerFlash] = useState(false)
  const [excludedTracks, setExcludedTracks] = useState<Set<string>>(new Set())
  const [metadataLookupAlbumIndex, setMetadataLookupAlbumIndex] = useState<number | null>(null)

  const [activeImport, setActiveImport] = useState<Import | null>(null)

  const trackNumberRefs = useRef(new Map<string, HTMLInputElement>())
  const bannerRef = useRef<HTMLDivElement>(null)

  function refreshList() {
    listImports()
      .then(setImports)
      .catch((err: unknown) => setListError(errorMessage(err)))
  }

  function toggleHistoryExpanded() {
    const next = !historyExpanded
    setHistoryExpanded(next)
    setHistoryVisibleCount(HISTORY_PAGE_SIZE)
    localStorage.setItem(HISTORY_EXPANDED_KEY, String(next))
  }

  useEffect(() => {
    if (step === 'list') refreshList()
  }, [step])

  function startImport() {
    setPlan(null)
    setErrors([])
    setExcludedTracks(new Set())
    setPlanLoadError(null)
    setConfirmError(null)
    setSourceDescription('')
    setLibraryId(null)
    setLibraryListError(null)
    listLibraries()
      .then(setLibraries)
      .catch((err: unknown) => setLibraryListError(errorMessage(err)))
    setStep('library')
  }

  function chooseBrowse() {
    setStep('browse')
  }

  function startCreateLibrary() {
    setCreateName('')
    setCreateError(null)
    setStep('createLibrary')
  }

  async function handleCreateLibraryConfirm(rootPath: string) {
    setCreateSubmitting(true)
    setCreateError(null)
    try {
      const lib = await createLibrary(createName, rootPath)
      setLibraryId(lib.id)
      setStep('source')
    } catch (err) {
      setCreateError(errorMessage(err))
    } finally {
      setCreateSubmitting(false)
    }
  }

  function applyPlanResponse(result: PlanResponse) {
    setPlan(result.plan)
    setErrors(result.errors)
  }

  async function handleBrowseConfirm(path: string) {
    if (libraryId == null) return
    setPlanLoading(true)
    setPlanLoadError(null)
    try {
      const result = await buildPlan(libraryId, path)
      setSourceDescription(path)
      applyPlanResponse(result)
      setStep('review')
    } catch (err) {
      setPlanLoadError(errorMessage(err))
    } finally {
      setPlanLoading(false)
    }
  }

  function handleUploaded(result: PlanResponse) {
    setSourceDescription('Uploaded from device')
    applyPlanResponse(result)
    setStep('review')
  }

  async function revalidate(next: Plan) {
    setPlan(next)
    if (libraryId == null) return
    const { plan: filtered, indexMaps } = buildIncludedPlan(next, excludedTracks)
    try {
      const result = await validatePlan(libraryId, filtered)
      setPlan(mergeIncludedPlan(next, result.plan, indexMaps))
      setErrors(remapErrors(result.errors, indexMaps))
    } catch (err) {
      setConfirmError(errorMessage(err))
    }
  }

  function toggleTrackExcluded(albumIndex: number, trackIndex: number) {
    setExcludedTracks((prev) => {
      const next = new Set(prev)
      const key = trackKey(albumIndex, trackIndex)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function toggleAlbumExcluded(al: PlanAlbum, albumIndex: number) {
    const excludeAll = albumSelection(al, albumIndex, excludedTracks) === 'all'
    setExcludedTracks((prev) => {
      const next = new Set(prev)
      al.tracks.forEach((_, ti) => {
        const key = trackKey(albumIndex, ti)
        if (excludeAll) next.add(key)
        else next.delete(key)
      })
      return next
    })
  }

  function handleAlbumFieldChange(albumIndex: number, field: 'artist' | 'album' | 'year', value: string) {
    if (!plan) return
    setPlan(withAlbumField(plan, albumIndex, field, value))
  }

  function handleAlbumFieldBlur(albumIndex: number, field: 'artist' | 'album' | 'year', value: string) {
    if (!plan) return
    void revalidate(withAlbumField(plan, albumIndex, field, value))
  }

  function handleTrackFieldChange(albumIndex: number, trackIndex: number, field: 'title' | 'trackNumber', value: string) {
    if (!plan) return
    setPlan(withTrackField(plan, albumIndex, trackIndex, field, value))
  }

  function handleTrackFieldBlur(albumIndex: number, trackIndex: number, field: 'title' | 'trackNumber', value: string) {
    if (!plan) return
    void revalidate(withTrackField(plan, albumIndex, trackIndex, field, value))
  }

  function handleOverwrite(albumIndex: number, trackIndex: number) {
    if (!plan) return
    void revalidate(withOverwrite(plan, albumIndex, trackIndex, true))
  }

  function handleOverwriteAlbum(albumIndex: number) {
    if (!plan) return
    void revalidate(withOverwriteAlbum(plan, albumIndex))
  }

  function handleMetadataLookupApply(albumIndex: number, result: MetadataLookupApply) {
    if (!plan) return
    void revalidate(withMetadataLookup(plan, albumIndex, result))
  }

  function focusTrackNumber(albumIndex: number, trackIndex: number) {
    trackNumberRefs.current.get(`${albumIndex}:${trackIndex}`)?.focus()
  }

  function handleConfirmClick() {
    if (confirmBlocked) {
      bannerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' })
      setBannerFlash(true)
      setTimeout(() => setBannerFlash(false), 1000)
      return
    }
    void handleConfirm()
  }

  async function handleConfirm() {
    if (!plan || libraryId == null) return
    setConfirming(true)
    setConfirmError(null)
    const { plan: filtered, indexMaps } = buildIncludedPlan(plan, excludedTracks)
    try {
      const result = await confirmImport(libraryId, sourceDescription, filtered)
      if (result.errors) {
        setErrors(remapErrors(result.errors, indexMaps))
        return
      }
      if (result.import) {
        setActiveImport(result.import)
        setStep('copying')
      }
    } catch (err) {
      setConfirmError(errorMessage(err))
    } finally {
      setConfirming(false)
    }
  }

  useEffect(() => {
    if (step !== 'copying' || !activeImport) return
    let cancelled = false
    const poll = () => {
      getImport(activeImport.id)
        .then((imp) => {
          if (cancelled) return
          setActiveImport(imp)
          if (imp.status !== 'copying') setStep('done')
        })
        .catch(() => {
          /* transient poll failure — try again next tick */
        })
    }
    poll()
    const id = setInterval(poll, POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(id)
    }
    // Deliberately keyed on activeImport's id, not the object itself — each
    // poll tick replaces activeImport, and depending on the whole object
    // would tear down and restart this interval every tick instead of just
    // once when the import being watched changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, activeImport?.id])

  const totalTracks = plan ? includedTrackCount(plan, excludedTracks) : 0
  const relevantErrors = errors.filter((e) => !isExcluded(e.albumIndex, e.trackIndex, excludedTracks))
  const hasErrors = relevantErrors.length > 0
  const nothingSelected = plan != null && totalTracks === 0
  const confirmBlocked = hasErrors || nothingSelected

  return (
    <div className="page-shell wide">
      {step === 'list' && (
        <>
          <div className="library-topbar">
            <div>
              <p className="eyebrow">Household library</p>
              <h1>Import</h1>
              <p className="sub">Bring new music in — files are copied and organized automatically, your originals are never touched.</p>
            </div>
            <button type="button" className="btn-primary" onClick={startImport}>
              ＋ Import music
            </button>
          </div>

          {listError && <p className="library-load-error">{listError}</p>}

          <button type="button" className="history-toggle" onClick={toggleHistoryExpanded}>
            {historyExpanded ? 'Hide import history' : 'Show import history'}
          </button>

          {historyExpanded &&
            (imports.length === 0 && !listError ? (
              <div className="library-empty compact">No imports yet — import some music to start building your library.</div>
            ) : (
              <div className="history-list">
                {imports.slice(0, historyVisibleCount).map((imp) => (
                  <div className="history-row" key={imp.id}>
                    <span className={`history-dot ${imp.status}`} />
                    <span className="history-path" title={imp.sourceDescription}>
                      {imp.sourceDescription}
                    </span>
                    <span className="history-count">{imp.trackCount} tracks</span>
                    <span className="history-date">{formatDate(imp.createdAt)}</span>
                    {imp.status === 'failed' && imp.error && (
                      <span className="history-warn" title={imp.error}>
                        ⚠
                      </span>
                    )}
                  </div>
                ))}
                {imports.length > historyVisibleCount && (
                  <button
                    type="button"
                    className="history-more"
                    onClick={() => setHistoryVisibleCount(imports.length)}
                  >
                    Show {imports.length - historyVisibleCount} more
                  </button>
                )}
              </div>
            ))}
        </>
      )}

      {step === 'library' && (
        <>
          <div className="library-topbar">
            <div>
              <p className="eyebrow">Import music</p>
              <h1>Choose a library</h1>
              <p className="sub">Which library should this import copy into?</p>
            </div>
            <button type="button" className="btn-ghost" onClick={() => setStep('list')}>
              Cancel
            </button>
          </div>

          {libraryListError && <p className="library-load-error">{libraryListError}</p>}

          <div className="lib-grid">
            {libraries.map((lib) => (
              <button
                type="button"
                className="lib-card"
                key={lib.id}
                onClick={() => {
                  setLibraryId(lib.id)
                  setStep('source')
                }}
              >
                <div className="name">{lib.name}</div>
                <div className="path mono">{lib.rootPath}</div>
                <div className="meta">{lib.trackCount} tracks</div>
              </button>
            ))}
            <button type="button" className="lib-card new" onClick={startCreateLibrary}>
              <div className="glyph">＋</div>
              <div>Create a new library</div>
            </button>
          </div>
        </>
      )}

      {step === 'createLibrary' && (
        <SourceFolderPicker
          title="Create a library"
          description="Give it a name, and pick (or create) the folder opusflow should organize files into."
          confirmLabel="Create library →"
          confirmingLabel="Creating…"
          cancelLabel="Back"
          nameField={{ label: 'Library name', value: createName, onChange: setCreateName }}
          submitting={createSubmitting}
          submitError={createError}
          onCancel={() => setStep('library')}
          onConfirm={handleCreateLibraryConfirm}
        />
      )}

      {step === 'source' && (
        <>
          <div className="library-topbar">
            <div>
              <p className="eyebrow">Import music</p>
              <h1>Choose a source</h1>
            </div>
            <button type="button" className="btn-ghost" onClick={() => setStep('library')}>
              Cancel
            </button>
          </div>
          <div className="source-grid">
            <button type="button" className="source-card" onClick={chooseBrowse}>
              <div className="glyph">🖥</div>
              <h3>Browse a server folder</h3>
              <p>Pick a folder the server can already see (e.g. a downloads share on the NAS). Fastest — nothing leaves the server.</p>
            </button>
            <button type="button" className="source-card" onClick={() => setStep('upload')}>
              <div className="glyph">⬆</div>
              <h3>Upload from this device</h3>
              <p>Select a folder on your computer or phone. Files are copied to the server over the network, then imported the same way.</p>
            </button>
          </div>
        </>
      )}

      {step === 'browse' && (
        <SourceFolderPicker
          title="Import music from…"
          description="Pick the folder containing the files to bring in. They'll be copied, not moved."
          confirmLabel="Next: review plan →"
          confirmingLabel="Loading…"
          cancelLabel="Back"
          submitting={planLoading}
          submitError={planLoadError}
          onCancel={() => setStep('source')}
          onConfirm={handleBrowseConfirm}
        />
      )}

      {step === 'upload' && libraryId != null && (
        <UploadDropzone libraryId={libraryId} onBack={() => setStep('source')} onUploaded={handleUploaded} />
      )}

      {step === 'review' && plan && (
        <>
          <div className="library-topbar">
            <div>
              <p className="eyebrow">Import music</p>
              <h1>Review the plan</h1>
              <p className="sub">Nothing is copied until you confirm below.</p>
            </div>
          </div>

          <div
            ref={bannerRef}
            className={[confirmBlocked ? 'plan-banner' : 'plan-banner ok', bannerFlash ? 'flash' : ''].filter(Boolean).join(' ')}
          >
            {hasErrors
              ? `⚠ ${relevantErrors.length} track${relevantErrors.length === 1 ? '' : 's'} need${relevantErrors.length === 1 ? 's' : ''} attention — resolve to continue.`
              : nothingSelected
                ? 'Nothing selected — check at least one track to import.'
                : `Ready to import ${totalTracks} track${totalTracks === 1 ? '' : 's'}.`}
          </div>

          {plan.albums.map((al, albumIndex) => {
            const destDir = al.tracks[0]?.destPath.split('/').slice(0, -1).join('/') + '/'
            const conflictCount = al.tracks.filter((tr) => tr.conflict && !tr.overwrite).length
            const selection = albumSelection(al, albumIndex, excludedTracks)
            return (
              <div className="album-group" key={albumIndex}>
                <div className="album-group-head">
                  <input
                    type="checkbox"
                    className="album-select-all"
                    checked={selection === 'all'}
                    ref={(el) => {
                      if (el) el.indeterminate = selection === 'mixed'
                    }}
                    onChange={() => toggleAlbumExcluded(al, albumIndex)}
                    title={selection === 'none' ? 'Select all tracks in this album' : 'Deselect all tracks in this album'}
                  />
                  <div className="album-art-stub">♪</div>
                  <div className="album-group-title-wrap">
                    <div className="album-group-title">
                      <input
                        className="field-inline wide"
                        value={al.artist}
                        placeholder="Artist"
                        onChange={(e) => handleAlbumFieldChange(albumIndex, 'artist', e.target.value)}
                        onBlur={(e) => handleAlbumFieldBlur(albumIndex, 'artist', e.target.value)}
                      />
                      {' — '}
                      <input
                        className="field-inline wide"
                        value={al.album}
                        placeholder="Album"
                        onChange={(e) => handleAlbumFieldChange(albumIndex, 'album', e.target.value)}
                        onBlur={(e) => handleAlbumFieldBlur(albumIndex, 'album', e.target.value)}
                      />
                    </div>
                    <div className="album-group-sub">
                      Year{' '}
                      <input
                        className="field-inline"
                        value={al.year || ''}
                        onChange={(e) => handleAlbumFieldChange(albumIndex, 'year', e.target.value)}
                        onBlur={(e) => handleAlbumFieldBlur(albumIndex, 'year', e.target.value)}
                      />{' '}
                      · destination <span className="mono">{destDir}</span>
                      {selection === 'none' && <span className="album-none-selected"> · 0 tracks selected</span>}
                    </div>
                  </div>
                  <button type="button" className="btn-ghost album-lookup-metadata" onClick={() => setMetadataLookupAlbumIndex(albumIndex)}>
                    Look up metadata
                  </button>
                  {conflictCount > 1 && (
                    <button type="button" className="btn-bad album-overwrite-all" onClick={() => handleOverwriteAlbum(albumIndex)}>
                      Overwrite all {conflictCount} existing
                    </button>
                  )}
                </div>
                <table className="track-plan-table">
                  <tbody>
                    {al.tracks.map((tr, trackIndex) => {
                      const excluded = isExcluded(albumIndex, trackIndex, excludedTracks)
                      const err = excluded ? undefined : errorFor(errors, albumIndex, trackIndex)
                      const missing = err?.missing ?? []
                      const isConflict = Boolean(err?.conflict)
                      return (
                        <Fragment key={trackIndex}>
                          <tr className={[isConflict ? 'conflict-row' : '', excluded ? 'excluded' : ''].filter(Boolean).join(' ') || undefined}>
                            <td className="tp-check">
                              <input
                                type="checkbox"
                                checked={!excluded}
                                onChange={() => toggleTrackExcluded(albumIndex, trackIndex)}
                              />
                            </td>
                            <td className="tp-num">
                              <input
                                ref={(el) => {
                                  if (el) trackNumberRefs.current.set(`${albumIndex}:${trackIndex}`, el)
                                }}
                                className={missing.includes('trackNumber') ? 'field-inline narrow missing' : 'field-inline narrow'}
                                value={tr.trackNumber || ''}
                                placeholder="—"
                                onChange={(e) => handleTrackFieldChange(albumIndex, trackIndex, 'trackNumber', e.target.value)}
                                onBlur={(e) => handleTrackFieldBlur(albumIndex, trackIndex, 'trackNumber', e.target.value)}
                              />
                            </td>
                            <td className="tp-title">
                              <input
                                className={missing.includes('title') ? 'field-inline wide missing' : 'field-inline wide'}
                                value={tr.title}
                                placeholder={missing.includes('title') ? 'Enter title — missing from tags' : ''}
                                onChange={(e) => handleTrackFieldChange(albumIndex, trackIndex, 'title', e.target.value)}
                                onBlur={(e) => handleTrackFieldBlur(albumIndex, trackIndex, 'title', e.target.value)}
                              />
                            </td>
                            <td className="tp-dest mono">
                              {tr.destPath.split('/').pop()}
                              {tr.hasCorrectionFile && (
                                <span className="tp-correction-file" title="Includes a .wvc correction file">
                                  📎
                                </span>
                              )}
                            </td>
                            <td className="tp-status">
                              {err ? <span className="warn-dot">⚠</span> : <span className="ok-dot">●</span>}
                            </td>
                          </tr>
                          {isConflict && !tr.overwrite && (
                            <tr>
                              <td colSpan={5} className="conflict-note-cell">
                                <div className="conflict-note">
                                  <span className="path" title={tr.destPath}>
                                    This file already exists at <span className="mono">{tr.destPath}</span>
                                  </span>
                                  <div className="conflict-actions">
                                    <button type="button" className="btn-ghost" onClick={() => focusTrackNumber(albumIndex, trackIndex)}>
                                      Change track #
                                    </button>
                                    <button type="button" className="btn-bad" onClick={() => handleOverwrite(albumIndex, trackIndex)}>
                                      Overwrite existing
                                    </button>
                                  </div>
                                </div>
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )
          })}

          <div className="review-foot">
            <span className="hint">Corrections you make here are also written into each copy's own tags.</span>
            <div className="review-foot-actions">
              {confirmError && <span className="picker-status error">{confirmError}</span>}
              <button type="button" className="btn-ghost" onClick={() => setStep('list')}>
                Cancel
              </button>
              <button
                type="button"
                className={confirmBlocked ? 'btn-primary blocked' : 'btn-primary'}
                disabled={confirming}
                aria-disabled={confirmBlocked}
                onClick={handleConfirmClick}
              >
                {confirming ? 'Starting…' : `Confirm & import ${totalTracks} track${totalTracks === 1 ? '' : 's'}`}
              </button>
            </div>
          </div>

          {metadataLookupAlbumIndex !== null &&
            plan &&
            (() => {
              const al = plan.albums[metadataLookupAlbumIndex]
              return (
                <MetadataLookupModal
                  initialArtist={al.artist}
                  files={al.tracks.map((tr) => tr.destPath.split('/').pop() ?? '')}
                  onApply={(result) => handleMetadataLookupApply(metadataLookupAlbumIndex, result)}
                  onClose={() => setMetadataLookupAlbumIndex(null)}
                />
              )
            })()}
        </>
      )}

      {step === 'copying' && activeImport && (
        <>
          <div className="library-topbar">
            <div>
              <p className="eyebrow">Import music</p>
              <h1>Copying</h1>
            </div>
          </div>
          <div className="library-card">
            <div className="card-top">
              <div className="path-row">
                <span className="card-path">{activeImport.sourceDescription}</span>
                <span className="pill copying">
                  <span className="pulse" /> importing
                </span>
              </div>
            </div>
            <div className="meta-row">
              <span className="count">
                {activeImport.filesProcessed} / {activeImport.filesTotal || '?'} files copied
              </span>
            </div>
            <div className="progress-track">
              <div
                className="progress-fill"
                style={{
                  width: `${activeImport.filesTotal > 0 ? Math.min(100, (activeImport.filesProcessed / activeImport.filesTotal) * 100) : 0}%`,
                }}
              />
            </div>
          </div>
        </>
      )}

      {step === 'done' && activeImport && (
        <div className="done-card">
          <div className="done-glyph">{activeImport.status === 'failed' ? '✕' : '✓'}</div>
          <h2>
            {activeImport.status === 'failed'
              ? 'Import failed'
              : `Imported ${activeImport.trackCount} track${activeImport.trackCount === 1 ? '' : 's'}`}
          </h2>
          <p>
            {activeImport.status === 'failed'
              ? activeImport.error
              : `Originals in ${activeImport.sourceDescription} were left exactly as they were.`}
          </p>
          {activeImport.fileErrors.length > 0 && (
            <p className="done-file-errors">
              {activeImport.fileErrors.length} file{activeImport.fileErrors.length === 1 ? '' : 's'} couldn't be copied.
            </p>
          )}
          <div className="done-links">
            <Link to="/artists">View your library →</Link>
            <button type="button" className="link-button" onClick={startImport}>
              Import more →
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
