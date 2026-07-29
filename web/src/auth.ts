// auth.ts is web's half of TDR 022's token-backed API auth — the same
// role connection.ts plays in mobile/src/services: token storage plus the
// one place that knows what "unauthenticated" means for this app.

const TOKEN_KEY = 'opusflow_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

// unauthorizedHandler is registered once by the app shell (App.tsx) so
// that api/library.ts's request() — called from everywhere, not just the
// unlock screen — can report a 401 (a revoked or expired token) without
// every call site having to handle it individually. Clearing the token
// and re-showing the unlock screen both happen here, in one place.
let unauthorizedHandler: (() => void) | null = null

export function setUnauthorizedHandler(handler: (() => void) | null): void {
  unauthorizedHandler = handler
}

export function notifyUnauthorized(): void {
  clearToken()
  unauthorizedHandler?.()
}

// validateToken proves a candidate token actually works against this
// server before it's ever stored — GET /api/health requires auth once an
// install is bootstrapped (TDR 022), so succeeding against it is both a
// reachability check and a token check in one request, same as mobile's
// validateServerConnection.
export async function validateToken(token: string): Promise<boolean> {
  try {
    const res = await fetch('/api/health', { headers: { Authorization: `Bearer ${token}` } })
    return res.ok
  } catch {
    return false
  }
}
