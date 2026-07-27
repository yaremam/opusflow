import { useEffect, useState } from 'react'
import { errorMessage, getAbout, type About } from '../api/library'
import './AboutPage.css'

export default function AboutPage() {
  const [about, setAbout] = useState<About | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    getAbout()
      .then(setAbout)
      .catch((err: unknown) => setLoadError(errorMessage(err)))
  }, [])

  return (
    <div className="page-shell">
      <span className="about-mark">
        <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M9 18V5l10-2v12" stroke="#0c1613" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
          <circle cx="6" cy="18" r="3" stroke="#0c1613" strokeWidth="2" />
          <circle cx="16" cy="15" r="3" stroke="#0c1613" strokeWidth="2" />
        </svg>
      </span>

      <div className="page-head">
        <div>
          <p className="eyebrow">About</p>
          <h1>opusflow</h1>
          <p className="sub">
            A private, self-hosted music platform — unifies your local library with connected streaming accounts
            into one place.
          </p>
        </div>
      </div>

      {loadError && <p className="about-load-error">{loadError}</p>}

      {about && (
        <>
          <div className="build-card">
            <div className="build-row">
              <span className="label">Version</span>
              <span className="value accent mono">{about.version}</span>
            </div>
            <div className="build-row">
              <span className="label">Built</span>
              <span className="value mono">{about.buildDate || 'unknown'}</span>
            </div>
          </div>

          <div className="about-actions">
            <a className="btn-primary" href="https://github.com/yaremam/opusflow" target="_blank" rel="noreferrer">
              View source on GitHub
            </a>
            <a className="btn-ghost" href="/health">
              Check /health
            </a>
          </div>

          <p className="about-footnote">
            Reporting an issue? Include the version above — it pins the exact commit this build was made from, even
            between tagged releases.
            <br />
            <a href="https://github.com/yaremam/opusflow" target="_blank" rel="noreferrer">
              github.com/yaremam/opusflow
            </a>
          </p>
        </>
      )}
    </div>
  )
}
