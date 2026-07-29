import { useState } from 'react'
import './SettingsPage.css'

interface TokenItem {
  id: string
  name: string
  token: string
  createdAt: string
}

export default function SettingsPage() {
  const [tokens, setTokens] = useState<TokenItem[]>([
    {
      id: '1',
      name: 'Android Phone (Pixel 8)',
      token: 'opusflow_token_98f731a89b42e1',
      createdAt: '2026-07-29',
    },
  ])

  const [newlyGeneratedToken, setNewlyGeneratedToken] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const handleGenerateToken = () => {
    const randomHex = Array.from({ length: 16 }, () =>
      Math.floor(Math.random() * 16).toString(16)
    ).join('')
    const generatedToken = `opusflow_token_${randomHex}`
    setNewlyGeneratedToken(generatedToken)

    const newToken: TokenItem = {
      id: Date.now().toString(),
      name: `Mobile Device (${new Date().toLocaleDateString()})`,
      token: generatedToken,
      createdAt: new Date().toISOString().split('T')[0],
    }

    setTokens((prev) => [newToken, ...prev])
  }

  const handleCopy = async (text: string) => {
    try {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(text)
      } else {
        const textarea = document.createElement('textarea')
        textarea.value = text
        textarea.style.position = 'fixed'
        textarea.style.opacity = '0'
        document.body.appendChild(textarea)
        textarea.focus()
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
      }
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (err) {
      console.error('Failed to copy token:', err)
    }
  }

  const handleRevoke = (id: string) => {
    setTokens((prev) => prev.filter((t) => t.id !== id))
  }

  return (
    <div className="page-shell wide">
      <div className="settings-topbar">
        <div>
          <p className="eyebrow">Settings</p>
          <h1>System & Companion Devices</h1>
          <p className="sub">Manage server pairing credentials and companion mobile devices.</p>
        </div>
      </div>

      <div className="settings-section">
        <h2>📱 Mobile Device Pairing & API Tokens</h2>
        <p className="sec-desc">
          Generate an API pairing token to connect your OpusFlow Android Companion App to this server.
        </p>

        <button type="button" className="btn-primary" onClick={handleGenerateToken}>
          ＋ Generate new pairing token
        </button>

        {newlyGeneratedToken && (
          <div style={{ marginTop: '1.25rem' }}>
            <span className="pill complete">✓ New Token Created</span>
            <div className="token-gen-box">
              <span className="mono">{newlyGeneratedToken}</span>
              <button
                type="button"
                className="btn-ghost"
                onClick={() => handleCopy(newlyGeneratedToken)}
              >
                {copied ? 'Copied! ✓' : 'Copy token'}
              </button>
            </div>
            <p className="sub" style={{ fontSize: '0.82rem', marginTop: '0.5rem' }}>
              Input your server host URL and this token into the OpusFlow Mobile app.
            </p>
          </div>
        )}

        <h3 style={{ fontSize: '0.9rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'var(--mist-400)', marginTop: '2rem', marginBottom: '0.75rem' }}>
          Paired Devices ({tokens.length})
        </h3>

        <div className="token-list">
          {tokens.map((item) => (
            <div key={item.id} className="token-row">
              <div>
                <div className="device-name">{item.name}</div>
                <div className="device-meta">
                  Created {item.createdAt} • <span className="mono-sub">{item.token.slice(0, 18)}…</span>
                </div>
              </div>
              <button
                type="button"
                className="btn-bad"
                onClick={() => handleRevoke(item.id)}
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
