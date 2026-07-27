import { useState } from 'react'
import { NavLink, Outlet } from 'react-router'
import { PlayerProvider } from '../player/PlayerContext'
import MiniPlayer from './MiniPlayer'
import QueueDrawer from './QueueDrawer'
import './AppLayout.css'

const NAV_LINKS = [
  { to: '/', label: 'Home', end: true },
  { to: '/artists', label: 'Artists' },
  { to: '/albums', label: 'Albums' },
  { to: '/songs', label: 'Songs' },
  { to: '/import', label: 'Import' },
  { to: '/libraries', label: 'Libraries' },
  { to: '/about', label: 'About' },
]

export default function AppLayout() {
  const [queueOpen, setQueueOpen] = useState(false)

  return (
    <PlayerProvider>
      <div className="app-shell">
        <header className="app-header">
          <NavLink to="/" className="wordmark" end>
            <span className="mark">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path d="M9 18V5l10-2v12" stroke="#0c1613" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
                <circle cx="6" cy="18" r="3" stroke="#0c1613" strokeWidth="2" />
                <circle cx="16" cy="15" r="3" stroke="#0c1613" strokeWidth="2" />
              </svg>
            </span>
            opusflow
          </NavLink>
          <nav className="app-nav">
            {NAV_LINKS.map((link) => (
              <NavLink key={link.to} to={link.to} end={link.end} className={({ isActive }) => (isActive ? 'current' : '')}>
                {link.label}
              </NavLink>
            ))}
          </nav>
        </header>
        <Outlet />
        <QueueDrawer open={queueOpen} />
        <MiniPlayer queueOpen={queueOpen} onToggleQueue={() => setQueueOpen((prev) => !prev)} />
      </div>
    </PlayerProvider>
  )
}
