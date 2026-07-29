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

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleRevoke = (id: string) => {
    setTokens((prev) => prev.filter((t) => t.id !== id))
  }

  return (
    <div className="settings-page">
      <div className="settings-header">
        <h1>Settings</h1>
        <p>Manage server configuration, companion devices, and API credentials.</p>
      </div>

      <div className="settings-card">
        <h2>📱 Mobile Device Pairing & API Tokens</h2>
        <p>
          Generate a pairing token to connect your OpusFlow Android Companion App to this server instance.
        </p>

        <button className="btn-primary" onClick={handleGenerateToken}>
          + Generate New Mobile Pairing Token
        </button>

        {newlyGeneratedToken && (
          <div style={{ marginTop: 20 }}>
            <span style={{ fontSize: 12, fontWeight: 600, color: '#10b981' }}>
              ✅ NEW TOKEN GENERATED
            </span>
            <div className="token-display-box">
              <span className="token-code">{newlyGeneratedToken}</span>
              <button
                className="btn-secondary"
                onClick={() => handleCopy(newlyGeneratedToken)}
              >
                {copied ? 'Copied! ✓' : 'Copy Token'}
              </button>
            </div>
            <p style={{ fontSize: 12, color: '#9ca3af', margin: 0 }}>
              Enter your server URL (e.g. <code>{window.location.origin}</code>) and this token on your mobile device.
            </p>
          </div>
        )}

        <h3 style={{ fontSize: 14, fontWeight: 600, marginTop: 28, marginBottom: 12 }}>
          ACTIVE PAIRED DEVICES ({tokens.length})
        </h3>

        <div className="token-list">
          {tokens.map((item) => (
            <div key={item.id} className="token-item">
              <div>
                <strong style={{ fontSize: 14 }}>{item.name}</strong>
                <div style={{ fontSize: 12, color: '#9ca3af', marginTop: 2 }}>
                  Created on {item.createdAt} • <code>{item.token.slice(0, 18)}...</code>
                </div>
              </div>
              <button className="btn-danger" onClick={() => handleRevoke(item.id)}>
                Revoke Token
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
