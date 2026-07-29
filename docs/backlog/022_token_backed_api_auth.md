# User Story: Token-Backed API Auth

## 1. User Value Statement

As a **self-hosted opusflow admin**,
I want **the API to actually enforce the pairing tokens it issues, instead of a token that's generated client-side and never checked**,
So that **exposing my server to my phone away from home doesn't also mean exposing my whole library to anyone who finds the URL**.

---

## 2. Strict Acceptance Criteria

- **AC-1 (App-wide enforcement)**: Every `/api/*` route requires a valid Bearer token once at least one token exists. `GET /health` remains unauthenticated (liveness only, no data). A missing/invalid/revoked token returns `401`.
- **AC-2 (Token storage)**: Tokens are stored in a new `api_tokens` table as a bcrypt hash (per-token random salt, no shared/hardcoded salt) — never in plaintext. Each row has a name, `created_at`, and `last_used_at`.
- **AC-3 (Bootstrap)**: On first startup with an empty `api_tokens` table, the server generates one token, writes its plaintext once to a file on the data volume (`<DATA_DIR>/.opusflow_admin_token`, `0600` permissions), and stores its bcrypt hash. Until any token exists, the API is unauthenticated (a brand-new, not-yet-configured install).
- **AC-4 (Web unlock)**: When the web app has no stored token (or the stored one is rejected), it shows an unlock screen instead of any catalog data. A valid token is stored in the browser and sent as `Authorization: Bearer` on every subsequent request — same mechanism as mobile, no separate session/cookie system.
- **AC-5 (Named, revocable tokens)**: Web Settings' existing "Generate new pairing token" flow calls a real endpoint, creates a named row in `api_tokens`, and displays the plaintext token once (text + QR code, `{serverUrl, token}`). The existing "Paired Devices" list reads real rows; "Revoke" deletes a row, and a request bearing that token immediately starts failing.
- **AC-6 (Mobile pairing)**: Mobile's Connect screen defaults to a QR scanner (camera permission) that fills the server URL and token from a scanned code; "Enter manually" falls back to the existing two text fields. Every subsequent mobile API call sends the paired token as `Authorization: Bearer`.
- **AC-7 (Distinguishable failure)**: An unreachable server and a rejected token surface as different messages on both web's unlock screen and mobile's Connect screen (network failure vs. `401`), not one generic error.
- **AC-8 (No accounts)**: No user accounts, roles, or login identity are introduced — every valid token grants the same full access, matching opusflow's single-household model (`docs/vision.md`). No token expiry; revoke-only.
