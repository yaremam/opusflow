import { createPortal } from 'react-dom'
import './RemoveModal.css'

interface ModalScrimProps {
  label: string
  onClose: () => void
  children: React.ReactNode
  panelClassName?: string
}

// ModalScrim is the shared centered-dialog shell (backdrop + panel) for
// any modal that might be triggered from inside a clickable row — e.g. a
// react-router <Link> like SongsPage's song-row. Rendered via a portal
// into document.body, since without one the modal would be a DOM
// descendant of that row and pick up its hover/click styling.
//
// The portal alone isn't enough, though: React bubbles synthetic events
// through the *component* tree regardless of where a portal renders in
// the DOM, so a click anywhere in here still reaches the Link's own
// click/navigate handler unless something stops it — this scrim does
// that once, centrally, rather than leaving every caller to rediscover
// and reimplement it. Reuses RemoveModal.css's .modal-scrim/.modal-panel
// rules (the pre-existing shared shape) rather than redefining them.
export default function ModalScrim({ label, onClose, children, panelClassName }: ModalScrimProps) {
  return createPortal(
    <div
      className="modal-scrim"
      onClick={(e) => {
        e.stopPropagation()
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className={`modal-panel${panelClassName ? ` ${panelClassName}` : ''}`} role="dialog" aria-modal="true" aria-label={label}>
        {children}
      </div>
    </div>,
    document.body,
  )
}
