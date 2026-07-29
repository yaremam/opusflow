# User Story: Drop the Web Auth Gate, Keep Pairing as Device Bookkeeping

## 1. User Value Statement

As the **household admin running opusflow on my LAN**,
I want **the web app to stop demanding a pasted/unlocked pairing token in every browser and device**,
So that **I can use my own library day-to-day without friction, while mobile pairing still gives me a QR-scan onboarding flow and a "Paired Devices" list to see what's connected**.

## 2. Strict Acceptance Criteria

- AC-1: Visiting the web app never shows an unlock/token-entry screen — there is no locked state. `UnlockPage` and its gating in `App.tsx` are removed.
- AC-2: No `/api/*` route (or any other route) returns 401 for a missing, invalid, or revoked Bearer token. A request with no `Authorization` header behaves identically to one with a garbage token: both succeed.
- AC-3: `POST /api/tokens`, `GET /api/tokens`, and `DELETE /api/tokens/{id}` remain available and unauthenticated (like everything else) — Settings' "Paired Devices" list, token generation, and QR code display are unchanged in the UI.
- AC-4: `DELETE /api/tokens/{id}` no longer refuses to delete the last remaining token — that guard (issue #59 / PR #65) protected against a lockout that can no longer happen once nothing is enforced, and is removed.
- AC-5: A request bearing a Bearer token that matches a real row still updates that row's `last_used_at` (best-effort, never blocking) — the "last used" column on the Paired Devices list keeps meaning something.
- AC-6: The one-time bootstrap mechanism (`.opusflow_admin_token` file, `auth_bootstrap` table, `auth.Bootstrap`) is removed entirely — a fresh install starts with zero tokens and an already-open app, no bootstrap file to find.
- AC-7: Mobile's "Connect" / QR-pairing flow succeeds as soon as the entered/scanned server URL responds to `GET /health` — it no longer calls the Bearer-gated `/api/health` trick to distinguish "wrong token" from "unreachable," since that distinction no longer exists. A successful pairing still saves the token and sends it as `Authorization: Bearer` on subsequent requests (for AC-5's bookkeeping).
- AC-8: `README.md` gets an explicit callout: if this instance is ever port-forwarded or reverse-proxied to be reachable outside the LAN, the operator is responsible for putting their own auth in front of it (reverse proxy basic auth, Tailscale, etc.) — opusflow itself no longer gates anything.
