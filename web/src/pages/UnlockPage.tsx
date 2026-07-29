import { useState } from 'react'
import { setToken, validateToken } from '../auth'
import './UnlockPage.css'

interface UnlockPageProps {
  onUnlock: () => void
}

// UnlockPage is shown instead of the whole app shell whenever no valid
// token is stored (TDR 022 AC-4) — first boot, or after a 401 clears a
// revoked one. There's no account/password here, just the one token this
// install's api_tokens table already knows about; see the bootstrap file
// on the data volume for where the very first one comes from.
export default function UnlockPage({ onUnlock }: UnlockPageProps) {
  const [token, setTokenInput] = useState('')
  const [checking, setChecking] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!token.trim()) return
    setChecking(true)
    setError(null)

    const valid = await validateToken(token.trim())
    if (valid) {
      setToken(token.trim())
      onUnlock()
    } else {
      setError("That token wasn't accepted. Check it and try again.")
    }
    setChecking(false)
  }

  return (
    <div className="unlock-shell">
      <form className="unlock-card" onSubmit={handleSubmit}>
        <div className="unlock-badge">
          <svg width="26" height="26" viewBox="0 0 24 24" fill="none">
            <rect x="5" y="11" width="14" height="9" rx="2" stroke="currentColor" strokeWidth="1.6" />
            <path d="M8 11V7a4 4 0 0 1 8 0v4" stroke="currentColor" strokeWidth="1.6" />
          </svg>
        </div>
        <h1>Unlock OpusFlow</h1>
        <p className="unlock-sub">
          This install is locked until an admin token is entered. Find it on the machine running opusflow.
        </p>

        <label className="field-label" htmlFor="admin-token">
          Admin token
        </label>
        <input
          id="admin-token"
          className="field-input"
          placeholder="Paste the token from your data volume"
          value={token}
          onChange={(e) => setTokenInput(e.target.value)}
          autoFocus
          autoCapitalize="none"
          autoCorrect="off"
        />

        {error && <p className="unlock-error">{error}</p>}

        <button type="submit" className="btn-primary" disabled={checking || !token.trim()}>
          {checking ? 'Checking…' : 'Unlock'}
        </button>

        <div className="unlock-hint">
          Look for it at <b>&lt;DATA_DIR&gt;/.opusflow_admin_token</b>, or run:
          <br />
          <span className="unlock-hint-cmd">docker compose logs app | grep &quot;pairing token&quot;</span>
        </div>
      </form>
    </div>
  )
}
