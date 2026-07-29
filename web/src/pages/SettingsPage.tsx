import { useEffect, useState } from 'react'
import { QRCodeSVG } from 'qrcode.react'
import { createToken, deleteToken, errorMessage, listTokens, type ApiToken, type CreatedApiToken } from '../api/library'
import './SettingsPage.css'

export default function SettingsPage() {
  const [tokens, setTokens] = useState<ApiToken[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)
  const [justCreated, setJustCreated] = useState<CreatedApiToken | null>(null)
  const [copied, setCopied] = useState(false)
  const [revokeError, setRevokeError] = useState<string | null>(null)

  useEffect(() => {
    refresh()
  }, [])

  async function refresh() {
    try {
      setTokens(await listTokens())
      setLoadError(null)
    } catch (err) {
      setLoadError(errorMessage(err))
    }
  }

  async function handleGenerateToken() {
    setCreating(true)
    setCreateError(null)
    try {
      const name = `Device (${new Date().toLocaleDateString()})`
      const created = await createToken(name)
      setJustCreated(created)
      setTokens((prev) => [created, ...prev])
    } catch (err) {
      setCreateError(errorMessage(err))
    } finally {
      setCreating(false)
    }
  }

  async function handleCopy(text: string) {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy token:', err)
    }
  }

  async function handleRevoke(id: number) {
    setRevokeError(null)
    try {
      await deleteToken(id)
      setTokens((prev) => prev.filter((t) => t.id !== id))
      if (justCreated?.id === id) setJustCreated(null)
    } catch (err) {
      setRevokeError(errorMessage(err))
    }
  }

  const qrPayload = justCreated ? JSON.stringify({ serverUrl: window.location.origin, token: justCreated.token }) : ''

  return (
    <div className="page-shell wide">
      <div className="settings-topbar">
        <div>
          <p className="eyebrow">Settings</p>
          <h1>System &amp; Companion Devices</h1>
          <p className="sub">Manage server pairing credentials and companion mobile devices.</p>
        </div>
      </div>

      <div className="settings-section">
        <h2>📱 Mobile Device Pairing &amp; API Tokens</h2>
        <p className="sec-desc">
          Generate an API pairing token to connect your OpusFlow Android Companion App to this server.
        </p>

        <button type="button" className="btn-primary" onClick={handleGenerateToken} disabled={creating}>
          {creating ? 'Generating…' : '＋ Generate new pairing token'}
        </button>
        {createError && <p className="settings-error">{createError}</p>}

        {justCreated && (
          <div className="token-gen-box">
            <div className="token-gen-qr">
              <QRCodeSVG value={qrPayload} size={88} />
            </div>
            <div className="token-gen-detail">
              <span className="pill complete">✓ New Token Created</span>
              <span className="mono">{justCreated.token}</span>
              <button type="button" className="btn-ghost" onClick={() => handleCopy(justCreated.token)}>
                {copied ? 'Copied! ✓' : 'Copy token'}
              </button>
            </div>
          </div>
        )}
        {justCreated && (
          <p className="sub" style={{ fontSize: '0.82rem', marginTop: '0.5rem' }}>
            Scan the QR code from the OpusFlow mobile app's Connect screen, or copy the token above — it won't be
            shown again.
          </p>
        )}

        <h3 className="settings-subhead">Paired Devices ({tokens.length})</h3>

        {loadError && <p className="settings-error">{loadError}</p>}
        {revokeError && <p className="settings-error">{revokeError}</p>}

        <div className="token-list">
          {tokens.map((item) => (
            <div key={item.id} className="token-row">
              <div>
                <div className="device-name">{item.name}</div>
                <div className="device-meta">
                  Created {new Date(item.createdAt).toLocaleDateString()}
                  {item.lastUsedAt ? ` • last used ${new Date(item.lastUsedAt).toLocaleDateString()}` : ' • never used'}
                </div>
              </div>
              <button
                type="button"
                className="btn-bad"
                onClick={() => handleRevoke(item.id)}
                disabled={tokens.length <= 1}
                title={tokens.length <= 1 ? "Can't revoke your only pairing token — generate another first." : undefined}
              >
                Revoke token
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
