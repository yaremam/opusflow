# TDR 024: Drop the Web Auth Gate, Keep Pairing as Device Bookkeeping

## 1. Context & Architectural Requirements

TDR 022 gated every `/api/*` route (web included) behind a bearer token, justified by a specific threat: an opusflow instance reachable from the internet (port-forwarded or reverse-proxied, so a phone can reach it away from home) would otherwise be wide open to anyone who found the URL.

In practice (issue #60), this install is LAN-only — mobile is only ever used at home — so that threat doesn't currently apply, and the gate's actual effect has been pure friction: every fresh browser, incognito window, or new device needs the token pasted in again before the web app is usable at all.

Web and mobile share the exact same `/api/*` routes on the exact same server. There is no reliable way to enforce a token for "requests from mobile" while leaving "requests from web" ungated — the backend cannot tell them apart. So the only two coherent states are "enforced everywhere" (TDR 022, as shipped) or "enforced nowhere." Given the LAN-only reality, we choose the latter.

This does not touch `docs/vision.md`'s single-household, no-multi-tenancy model — that was never in question here; "user accounts" in the original issue was answered by scoping this to removing the friction, not adding per-person logins.

---

## 2. Alternatives Evaluated

### Alternative A: Keep enforcement, improve ergonomics with a long-lived cookie/session
Replace the manually-pasted localStorage token with a server-set session cookie that persists for months, so a returning browser doesn't need re-pairing.

- **Pros**: Never has a "forgot to re-enable it" failure mode if the deployment is later exposed to the internet — protection stays on by default.
- **Cons**: A second auth mechanism (cookie + session lifecycle) alongside mobile's Bearer tokens, for a threat that isn't live today. Doesn't fully remove the friction either — a *first* visit from any new browser/device still needs some out-of-band unlock step, which was the actual complaint.

### Alternative B: Auto-detect LAN vs. non-LAN requests, enforce only on the latter
Backend inspects the request's source address; enforces the token only for requests that don't look local.

- **Pros**: Removes the pain for LAN use while, in theory, still closing the internet-exposure gap automatically.
- **Cons**: "Looks local" is unreliable behind Docker's own networking/NAT and any reverse proxy — the exact deployment shape TDR 022 was originally worried about is the one most likely to break this detection. Adds real complexity (a second code path, a new way to be silently wrong) to guard against a threat that isn't live for this install today.

### Alternative C (Chosen): Drop enforcement entirely; keep tokens as pairing/bookkeeping only
Remove the auth middleware's blocking behavior, the bootstrap mechanism, and the web unlock screen. Keep `/api/tokens` CRUD and mobile's QR-pairing flow exactly as they are today, but the token stops being checked for access control — it's now identity/bookkeeping (drives the "last used" column on Settings' Paired Devices list) rather than a gate.

- **Pros**: Directly solves the reported friction with no new mechanism to build or maintain. Matches how most self-hosted home-server apps handle this exact tradeoff: open by default on the LAN, operator's responsibility to front it with their own auth (reverse proxy, Tailscale) if they choose to expose it. Deletes code (bootstrap file/marker, the last-token-deletion guard from issue #59) rather than adding more.
- **Cons**: If this instance is later port-forwarded or reverse-proxied without the operator adding their own auth in front of it, the whole library is open to anyone who finds the URL — the exact gap TDR 022 closed. Mitigated with an explicit README callout (AC-8), not code.

---

## 3. Structural Decision

We select **Alternative C**.

1. **`auth.Middleware`**: stops returning 401. On a request bearing a Bearer token that matches a real row, it still calls `ValidateAndTouch` best-effort to keep `last_used_at` accurate; on anything else (missing, invalid, revoked, or no header at all), it just lets the request through. No more bootstrap-window/fail-closed logic — there's nothing left to fail closed on.
2. **Bootstrap removal**: `auth.Bootstrap`, `BootstrapFileName`, the `auth_bootstrap` table, and its migration are deleted. `cmd/server/main.go`'s startup goroutine drops the `auth.Bootstrap(...)` call.
3. **Last-token-deletion guard removal**: `Service.DeleteToken`'s `ErrLastToken` check (issue #59 / PR #65) is removed — deleting every token can no longer lock anyone out of anything, so the guard has nothing left to protect against. `handleDeleteToken` drops its 409 branch.
4. **Web**: `UnlockPage.tsx` and its route are deleted. `App.tsx` drops the `unlocked` state and gating entirely — the app always renders. `auth.ts` drops `getToken`/`setToken`/`clearToken`/`setUnauthorizedHandler`/`notifyUnauthorized` and the token-gating concept; `api/library.ts`'s `request()` stops attaching an `Authorization` header and stops special-casing 401. Settings' token generation/list/revoke/QR UI is unchanged — it's still how a household admin mints a token to hand to a new device.
5. **Mobile**: `validateServerConnection` drops the Bearer-gated `/api/health` trick and instead confirms pairing by hitting the always-open `GET /health` — a successful pairing means "this URL is a reachable opusflow server," full stop. `ConnectionResult`'s `'unauthorized'` variant is removed (nothing can produce it anymore). On success, the token is still saved and sent as `Authorization: Bearer` on every subsequent request purely so `ValidateAndTouch` keeps the Paired Devices "last used" column meaningful — it's identity, not a credential being checked.
6. **README**: gets an explicit callout (AC-8) that exposing this instance beyond the LAN is the operator's responsibility to secure (reverse proxy auth, Tailscale, etc.) — opusflow no longer gates anything itself.

---

## 4. Cross-Workspace Implications

- **`backend/`**: `internal/auth/middleware.go` (behavior change, no more 401s), `internal/auth/bootstrap.go` (deleted), `internal/auth/service.go` (`ErrLastToken`/ its check removed), `internal/httpserver/tokens.go` (409 branch removed), `internal/db/migrations/0010_auth_bootstrap_marker.sql`'s table gets a new migration dropping it, `cmd/server/main.go` (drops the `auth.Bootstrap` call and the `authStore` parameter it fed for gating purposes — `authStore` itself stays, since `/api/tokens` CRUD and the touch-on-request behavior still need it).
- **`web/`**: `src/pages/UnlockPage.tsx` (+ `.css`) deleted; `src/App.tsx`, `src/auth.ts`, `src/api/library.ts` lose their gating logic. `SettingsPage.tsx` unchanged.
- **`mobile/`**: `src/services/connection.ts` (`validateServerConnection` simplified, `ConnectionResult` loses `'unauthorized'`), `src/screens/ConnectScreen.tsx` (drops the now-impossible "wrong token" status message branch — PR #66's scan-failure overlay logic still applies, just with one fewer failure mode to render). The token is still stored and sent on every request (AC-5/AC-7), so `SecureStore`-backed credential storage and the `Authorization: Bearer` header on catalog/stream requests are unchanged.
- **Docs**: `README.md` gets AC-8's callout. `docs/ARCHITECTURE.md` should be updated once implementation lands to reflect that `/api/*` is open again, matching how it described the pre-TDR-022 state, with a note on why (this TDR) and what's still there (pairing/bookkeeping tokens).
- **Supersedes**: TDR 022's Alternative B (app-wide enforcement) is reversed here in light of the LAN-only reality established in this TDR's Context section — TDR 022 remains correct for the threat model it was written under (an internet-exposed instance), which is why AC-8 exists rather than deleting that reasoning.
