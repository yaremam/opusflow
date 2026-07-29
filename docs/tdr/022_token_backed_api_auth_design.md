# TDR 022: Token-Backed API Auth

## 1. Context & Architectural Requirements

The backend has never had any auth — every `/api/*` route is open to anyone who can reach it. TDR 019's Android companion app introduced the *concept* of a "pairing token" (AC-1: "allow entering a server host URL and pairing token"), but nothing ever backed it: web's Settings page generates a `Math.random()` string held only in `useState` (gone on refresh, never sent anywhere), and mobile sends it as a Bearer header that the backend never checks. The token exists as UI, not as security.

This matters specifically because of *why* someone pairs a phone at all: to reach their library from outside their home network. A self-hosted deployment that's reachable from the internet (port-forwarded or behind a reverse proxy, so a phone can reach it away from home) is, today, reachable by anyone who finds the URL — the pairing token adds no actual protection.

opusflow is explicitly single-household (`docs/vision.md`, "Household model" — one shared library per install, no multi-tenancy). This rules out an accounts/roles system; the only meaningful distinction is "has a valid token" vs. not.

---

## 2. Alternatives Evaluated

### Alternative A: Mobile-only enforcement
Validate the token once at pairing time via a dedicated endpoint; leave the existing catalog/stream routes exactly as open as today for everyone.

- **Pros**: Smallest change; web needs no unlock step; matches the narrowest reading of TDR 019 ("mobile app enters a token").
- **Cons**: Not real security — anyone who skips the mobile app and hits `/api/library/*` directly gets everything, unauthenticated, exactly as today. Doesn't close the actual gap (an internet-exposed instance stays exposed). Given the user explicitly asked for the *real* thing after learning the current token is fake, shipping another cosmetic layer defeats the point.

### Alternative B (Chosen): App-wide enforcement, one shared token mechanism
Every `/api/*` route requires a valid Bearer token, checked against a single `api_tokens` table. Web is a token holder exactly like a phone is — it stores a token in the browser and sends it as `Authorization: Bearer` on every request, no separate cookie/session system. Bootstrapped via a one-time file on the data volume.

- **Pros**: Actually closes the gap described above — every route is equally protected regardless of client. One code path (one middleware, one table, one validation function) for both clients, instead of two different auth mechanisms to maintain. Reuses the "device pairing" mental model the UI already has (Settings' "Paired Devices" list) rather than inventing a parallel accounts concept.
- **Cons**: Web needs a new "locked" state it's never had before (a one-time unlock screen on a fresh browser/device). Bootstrapping the very first token needs an out-of-band channel (can't display a QR code from a web app that's itself locked).

### Alternative C: httpOnly session cookie for web, separate Bearer tokens for mobile
Web authenticates via a server-set cookie (with CSRF protection); mobile keeps using Bearer tokens against a separate table or the same one.

- **Pros**: Cookies are the conventional browser session mechanism; avoids web manually attaching an `Authorization` header on every fetch.
- **Cons**: Two different auth mechanisms (cookie+CSRF vs. Bearer) to implement, test, and reason about for a single-household app where the extra complexity buys nothing — there's no cross-site request to forge against, no third-party browser context, no session expiry model that means anything here. Alternative B's one mechanism for both clients is simpler and exactly as secure for this threat model.

---

## 3. Structural Decision

We select **Alternative B**.

1. **Token storage**: new `api_tokens` table — `id`, `name`, `token_hash` (bcrypt, per-row random salt via `golang.org/x/crypto/bcrypt` — no hardcoded/shared salt), `created_at`, `last_used_at`. Plaintext tokens are never stored; they're shown once at creation time (web) or scanned once (mobile) and never retrievable again.
2. **Middleware**: a single auth middleware wraps every route except `GET /health`. It's a no-op (allows all requests) while `api_tokens` is empty — the bootstrap window — and requires a valid `Authorization: Bearer <token>` matching some row's hash once at least one row exists. A `401` on missing/invalid/revoked distinctly from a network-level failure, so both web and mobile can tell "wrong token" from "can't reach the server" apart.
3. **Bootstrap**: `cmd/server` checks `api_tokens` on startup; if empty, generates a token (`crypto/rand`, sufficient entropy that brute-force isn't a practical concern — no additional rate-limiting planned for this pass), inserts its hash, and writes the plaintext once to `<DATA_DIR>/.opusflow_admin_token` with `0600` permissions — the same filesystem-trust posture this repo already uses for Postgres (no password, `POSTGRES_HOST_AUTH_METHOD=trust`; ARCHITECTURE.md §on Postgres config). Not regenerated on subsequent boots once any token exists.
4. **Web**: a new unlock screen (shown whenever no valid token is in browser storage, or the stored one gets a `401`) replaces the app shell until a token's entered; from then on every `web/src/api/library.ts` request carries it. Settings' pairing section becomes real: `POST` to create a named token (returns the plaintext once, rendered as text + a QR code encoding `{serverUrl, token}`), `GET` to list existing rows (name, created/last-used, never the token itself again), `DELETE` to revoke.
5. **Mobile**: Connect screen's default view is a QR scanner (`expo-camera`) that decodes `{serverUrl, token}` directly into `connection.ts`'s stored credentials; "Enter manually" drops back to the existing two-field form. `validateServerConnection` now calls the (now-authenticated) `/api/health` with the candidate token — success proves both reachability and a valid token in one round trip, replacing the old unauthenticated-`/health`-fallback logic that predates real auth existing.

---

## 4. Cross-Workspace Implications

- **`backend/`**: new migration for `api_tokens`; new `internal/library` (or a new `internal/auth` package) for token generation/hashing/validation; new middleware wired into `httpserver.NewHandler`; new `POST/GET/DELETE /api/tokens` routes; bootstrap logic in `cmd/server`. `golang.org/x/crypto/bcrypt` becomes a new dependency (no external service, pure Go, already in the broader `x/crypto` module tree Go projects commonly pull in for exactly this).
- **`web/`**: new unlock screen/route guard; `api/library.ts`'s `request()` helper gains the `Authorization` header and centralized 401-handling (redirect to unlock); `SettingsPage.tsx` rewritten from local `useState` demo data to real fetch/create/revoke calls; a QR-generation dependency for the token-display view (client-side, no external service call — matches this repo's existing use of `sharp` for image work, though QR encoding is a different, lightweight library).
- **`mobile/`**: new `expo-camera` dependency for QR scanning; `ConnectScreen.tsx` restructured (scan-first, manual fallback); `connection.ts`'s `validateServerConnection` changes which endpoint it calls and how it interprets failure (401 vs. network error) now that `/api/health` requires auth.
- **No schema changes to existing tables.** No change to `docs/vision.md`'s household model — this is infrastructure for the existing single-shared-library posture, not a step toward multi-tenancy (see AC-8).
