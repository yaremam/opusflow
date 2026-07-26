import { NavLink, Outlet } from 'react-router'
import './AppLayout.css'

const NAV_LINKS = [
  { to: '/', label: 'Home', end: true },
  { to: '/artists', label: 'Artists' },
  { to: '/albums', label: 'Albums' },
  { to: '/songs', label: 'Songs' },
  { to: '/import', label: 'Import' },
  { to: '/libraries', label: 'Libraries' },
]

export default function AppLayout() {
  return (
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
    </div>
  )
}
