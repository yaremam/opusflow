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
import SourceFolderPicker from '../components/SourceFolderPicker'
import UploadDropzone from '../components/UploadDropzone'
import './ImportPage.css'

type Step = 'list' | 'library' | 'createLibrary' | 'source' | 'browse' | 'upload' | 'review' | 'copying' | 'done'

const POLL_INTERVAL_MS = 1200

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function trackCount(plan: Plan): number {
  return plan.albums.reduce((sum, al) => sum + al.tracks.length, 0)
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

function errorFor(errors: ValidationError[], albumIndex: number, trackIndex: number): ValidationError | undefined {
  return errors.find((e) => e.albumIndex === albumIndex && e.trackIndex === trackIndex)
}

export default function ImportPage() {
  const [step, setStep] = useState<Step>('list')

  const [imports, setImports] = useState<Import[]>([])
  const [listError, setListError] = useState<string | null>(null)

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

  const [activeImport, setActiveImport] = useState<Import | null>(null)

  const trackNumberRefs = useRef(new Map<string, HTMLInputElement>())

  function refreshList() {
    listImports()
      .then(setImports)
      .catch((err: unknown) => setListError(errorMessage(err)))
  }

  useEffect(() => {
    if (step === 'list') refreshList()
  }, [step])

  function startImport() {
    setPlan(null)
    setErrors([])
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
    try {
      const result = await validatePlan(libraryId, next)
      setPlan(result.plan)
      setErrors(result.errors)
    } catch (err) {
      setConfirmError(errorMessage(err))
    }
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

  function focusTrackNumber(albumIndex: number, trackIndex: number) {
    trackNumberRefs.current.get(`${albumIndex}:${trackIndex}`)?.focus()
  }

  async function handleConfirm() {
    if (!plan || libraryId == null) return
    setConfirming(true)
    setConfirmError(null)
    try {
      const result = await confirmImport(libraryId, sourceDescription, plan)
      if (result.errors) {
        setErrors(result.errors)
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

  const totalTracks = plan ? trackCount(plan) : 0
  const hasErrors = errors.length > 0

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

          {imports.length === 0 && !listError ? (
            <div className="library-empty">No imports yet — import some music to start building your library.</div>
          ) : (
            <div className="library-list">
              {imports.map((imp) => (
                <div className="library-card" key={imp.id}>
                  <div className="card-top">
                    <div className="path-row">
                      <span className="card-path">{imp.sourceDescription}</span>
                      <span className={`pill ${imp.status}`}>
                        {imp.status === 'copying' && <span className="pulse" />}
                        {imp.status}
                      </span>
                    </div>
                  </div>
                  {imp.status === 'copying' ? (
                    <>
                      <div className="progress-track">
                        <div
                          className="progress-fill"
                          style={{ width: `${imp.filesTotal > 0 ? Math.min(100, (imp.filesProcessed / imp.filesTotal) * 100) : 0}%` }}
                        />
                      </div>
                      <div className="meta-row">
                        <span className="count">
                          {imp.filesProcessed} / {imp.filesTotal || '?'} files copied
                        </span>
                      </div>
                    </>
                  ) : (
                    <div className="meta-row">
                      <span className="count">{imp.trackCount} tracks</span>
                      <span>Imported {formatDate(imp.createdAt)}</span>
                    </div>
                  )}
                  {imp.status === 'failed' && imp.error && <div className="error-note">{imp.error}</div>}
                </div>
              ))}
            </div>
          )}
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

          <div className={hasErrors ? 'plan-banner' : 'plan-banner ok'}>
            {hasErrors
              ? `⚠ ${errors.length} track${errors.length === 1 ? '' : 's'} need${errors.length === 1 ? 's' : ''} attention — resolve to continue.`
              : `Ready to import ${totalTracks} track${totalTracks === 1 ? '' : 's'}.`}
          </div>

          {plan.albums.map((al, albumIndex) => {
            const destDir = al.tracks[0]?.destPath.split('/').slice(0, -1).join('/') + '/'
            return (
              <div className="album-group" key={albumIndex}>
                <div className="album-group-head">
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
                    </div>
                  </div>
                </div>
                <table className="track-plan-table">
                  <tbody>
                    {al.tracks.map((tr, trackIndex) => {
                      const err = errorFor(errors, albumIndex, trackIndex)
                      const missing = err?.missing ?? []
                      const isConflict = Boolean(err?.conflict)
                      return (
                        <Fragment key={trackIndex}>
                          <tr className={isConflict ? 'conflict-row' : undefined}>
                            <td className="tp-num">
                              <input
                                ref={(el) => {
                                  if (el) trackNumberRefs.current.set(`${albumIndex}:${trackIndex}`, el)
                                }}
                                className={missing.includes('trackNumber') ? 'field-inline missing' : 'field-inline'}
                                style={{ width: '3ch' }}
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
                            <td className="tp-dest mono">{tr.destPath.split('/').pop()}</td>
                            <td className="tp-status">
                              {err ? <span className="warn-dot">⚠</span> : <span className="ok-dot">●</span>}
                            </td>
                          </tr>
                          {isConflict && !tr.overwrite && (
                            <tr>
                              <td colSpan={4} className="conflict-note-cell">
                                <div className="conflict-note">
                                  <span className="path">
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
              <button type="button" className="btn-primary" disabled={hasErrors || confirming} onClick={handleConfirm}>
                {confirming ? 'Starting…' : `Confirm & import ${totalTracks} track${totalTracks === 1 ? '' : 's'}`}
              </button>
            </div>
          </div>
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
