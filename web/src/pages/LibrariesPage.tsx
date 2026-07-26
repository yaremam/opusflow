import { useEffect, useState } from 'react'
import { createLibrary, deleteLibrary, errorMessage, listLibraries, type Library } from '../api/library'
import RemoveModal from '../components/RemoveModal'
import SourceFolderPicker from '../components/SourceFolderPicker'
import './LibrariesPage.css'

export default function LibrariesPage() {
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)

  const [creating, setCreating] = useState(false)
  const [createName, setCreateName] = useState('')
  const [createSubmitting, setCreateSubmitting] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  const [removingLib, setRemovingLib] = useState<Library | null>(null)
  const [removeSubmitting, setRemoveSubmitting] = useState(false)
  const [removeError, setRemoveError] = useState<string | null>(null)

  function refresh() {
    listLibraries()
      .then(setLibraries)
      .catch((err: unknown) => setLoadError(errorMessage(err)))
  }

  useEffect(refresh, [])

  async function handleCreateConfirm(rootPath: string) {
    setCreateSubmitting(true)
    setCreateError(null)
    try {
      await createLibrary(createName, rootPath)
      setCreating(false)
      setCreateName('')
      refresh()
    } catch (err) {
      setCreateError(errorMessage(err))
    } finally {
      setCreateSubmitting(false)
    }
  }

  async function handleDelete(deleteFiles: boolean) {
    if (!removingLib) return
    setRemoveSubmitting(true)
    setRemoveError(null)
    try {
      await deleteLibrary(removingLib.id, deleteFiles)
      setRemovingLib(null)
      refresh()
    } catch (err) {
      setRemoveError(errorMessage(err))
      setRemoveSubmitting(false)
    }
  }

  return (
    <div className="page-shell wide">
      <div className="library-topbar">
        <div>
          <p className="eyebrow">Settings</p>
          <h1>Libraries</h1>
          <p className="sub">Every library opusflow organizes music into.</p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setCreating(true)}>
          ＋ Create library
        </button>
      </div>

      {loadError && <p className="library-load-error">{loadError}</p>}

      {libraries.length === 0 && !loadError ? (
        <div className="library-empty">No libraries yet — create one to start importing music.</div>
      ) : (
        <div className="lib-list">
          {libraries.map((lib) => (
            <div className="lib-row" key={lib.id}>
              <div className="info">
                <div className="name">{lib.name}</div>
                <div className="path mono">{lib.rootPath}</div>
                <div className="meta">{lib.trackCount} tracks</div>
              </div>
              <button
                type="button"
                className="btn-icon"
                title={`Delete ${lib.name}`}
                aria-label={`Delete ${lib.name}`}
                onClick={() => setRemovingLib(lib)}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}

      {creating && (
        <SourceFolderPicker
          title="Create a library"
          description="Give it a name, and pick (or create) the folder opusflow should organize files into."
          confirmLabel="Create library →"
          confirmingLabel="Creating…"
          nameField={{ label: 'Library name', value: createName, onChange: setCreateName }}
          submitting={createSubmitting}
          submitError={createError}
          onCancel={() => {
            setCreating(false)
            setCreateName('')
            setCreateError(null)
          }}
          onConfirm={handleCreateConfirm}
        />
      )}

      {removingLib && (
        <RemoveModal
          name={removingLib.name}
          description={`This removes its ${removingLib.trackCount} track${removingLib.trackCount === 1 ? '' : 's'} from your library. You can also delete opusflow's copies of these files from disk — your choice, every time.`}
          submitting={removeSubmitting}
          submitError={removeError}
          onDeleteFiles={() => handleDelete(true)}
          onKeepFiles={() => handleDelete(false)}
          onCancel={() => {
            setRemovingLib(null)
            setRemoveError(null)
          }}
        />
      )}
    </div>
  )
}
