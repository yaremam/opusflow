# TDR 009: About Page & Build Versioning

## 1. Context & Architectural Requirements

[TDR 004](004_self_hosted_deployment_design.md) gave every build a git-SHA
identity: `GIT_SHA` is stamped into the Docker image at build time (both as
an `ENV` and an OCI `org.opencontainers.image.revision` label), and `GET
/health` surfaces it as `{"status":"ok","revision":"<sha>"}` so a self-hoster
can tell which commit a running container was built from. That mechanism
works, but it's SHA-only — there's no notion of a release, no way to answer
"is this roughly the same build as last week" without diffing two opaque
hashes, and no UI surface for it at all (checking means running `curl
/health` from a terminal). This feature (grilled and mocked up, signed off
before this doc) adds a real semantic version on top of the existing SHA
mechanism and a dedicated in-app About page to display it, reachable from the
top navigation.

The project has never had a version number before this — no `VERSION` file,
no git tags, no semver anywhere in the repo. Nightly builds are the only
build channel (`.github/workflows/nightly.yml`), tagged only by date and
short SHA (`:nightly-<date>`, `:sha-<short>`); there is no existing release
process to hook a version bump into.

## 2. Alternatives Evaluated

### Alternative: version source — a `VERSION` file vs. git tags

- **`VERSION` file in the repo root** — Pros: no dependency on git metadata
  being present in the build context (relevant for shallow clones); trivial
  to read from any language/tool. Cons: a second source of truth alongside
  git history that has to be remembered and bumped by hand on every release,
  with nothing enforcing it's ever updated — drifts silently.
- **Git tags (chosen)** — Pros: the version *is* a specific commit, by
  construction — no separate bump-and-commit step to forget, and `git
  describe` gives both "nearest release" and "exact commit" in one string.
  Cons: requires actually creating annotated tags as a deliberate step (no
  tags exist in this repo yet — closed by AC-2, tagging `v0.1.0` as part of
  this feature); a shallow `git clone` without tag history would break `git
  describe` (mitigated in CI by fetching full history/tags before build; a
  plain local `go run` without any `.git` at all falls back to `"dev"` per
  AC-4).

### Alternative: version format for builds between tags — full `git describe` vs. nearest-tag-only

- **Nearest tag only** (e.g. always `v0.1.0` until the next tag) — Pros:
  cleaner, shorter string to display. Cons: every nightly build between
  releases would report an identical version, indistinguishable from each
  other in the one place (the About page) meant to identify a build — the
  existing `/health` revision field already solves "which exact commit," so
  dropping that precision here would be a regression for the nightly-heavy
  way this project actually ships.
- **Full `git describe --tags --always` (chosen)** — e.g.
  `v0.1.0-4-gabc123f` for the 4th commit past `v0.1.0`, or bare `v0.1.0`
  exactly on a tag. Pros: one string carries both "roughly what release this
  is near" and "exact commit," matching what `/health`'s revision field
  already gave, so nothing is lost by consolidating onto this one value
  (§3). Cons: uglier string than a bare tag — accepted, this is a
  build-identity string for bug reports, not user-facing marketing copy.

### Alternative: where version/build-date live in the API, and what happens to `/health`'s existing `revision` field

- **Extend `/health`** (add `version`/`buildDate` fields, keep `revision`
  too) — Pros: no new endpoint, no change to `/health`'s existing contract.
  Cons: `/health` is a liveness/readiness check (already documented as such
  by its own name and by the nightly workflow's `skopeo` skip-check reading
  the *image label*, never this endpoint) — growing it into "everything
  about this build" conflates two different concerns, and duplicates the
  same commit identity twice in one response once `version` already embeds
  it (previous alternative).
- **New `GET /api/about`, `/health` drops `revision` (chosen)** — Pros:
  `/health` goes back to a minimal liveness signal (`{"status":"ok"}`);
  `/api/about` becomes the one place build identity lives, matching the
  page it exists to feed. Cons: a real, visible contract change to an
  endpoint TDR 004 shipped and `docs/deploy/synology.md` already told
  self-hosters to check — every reference to `/health`'s `revision` field
  (README, that doc, `deploy/docker-compose.yml`'s comment,
  `docs/ARCHITECTURE.md`) is updated by this feature (§4) rather than left
  stale, and the existing `TestHealthReportsRevision`-shaped assertion in
  `httpserver_test.go` is updated to match the new `{"status":"ok"}`-only
  response.

## 3. Structural Decision

**Tag**: an annotated tag `v0.1.0` is created on `main`'s current tip and
pushed, establishing the convention (AC-2).

**Dockerfile**: gains a `VERSION` build-arg (default `dev`) and a
`BUILD_DATE` build-arg (default empty), stamped as `ENV` vars the same way
`GIT_SHA` already is — additive, same final layer `GIT_SHA` already touches
so no new layer is introduced.

**`.github/workflows/nightly.yml`**: the `publish` job computes `VERSION` via
`git describe --tags --always` and `BUILD_DATE` via `date -u
+%Y-%m-%dT%H:%M:%SZ`, passing both as `--build-arg` alongside the existing
`GIT_SHA` one. The `changes` job's fetch step is checked to ensure tag
history is actually present (`fetch-depth: 0` or equivalent) — a shallow
checkout would make `git describe --tags` fall back to `--always`'s bare-SHA
behavior silently, which would work but defeat the point.

**Backend**: `cmd/server/main.go` reads `VERSION`/`BUILD_DATE` env vars
(defaulting to `"dev"`/`""` the same way `GIT_SHA` already defaults, for
local `go run`). New `GET /api/about` returns `{"version", "buildDate"}`.
`GET /health`'s `healthResponse` drops `Revision`; `GIT_SHA`/`revision` is no
longer threaded into `httpserver.New` at all — `VERSION`/`BUILD_DATE` are
instead.

**Frontend**: new `AboutPage.tsx` (route `/about`), reachable from a new,
last entry in `AppLayout.tsx`'s `NAV_LINKS`. Fetches `GET /api/about` and
renders version + build date + a static link to
`github.com/yaremam/opusflow` (matching the Dockerfile's existing
`org.opencontainers.image.source` label — not derived from the API, since
the repo URL doesn't vary by build). Built from the existing design tokens
in `web/src/index.css` (`page-shell`, `eyebrow`, existing color/shadow
tokens) — no new palette or component system, per the signed-off mockup.

## 4. Cross-Workspace Implications

- **`backend/`**:
  - `backend/cmd/server/main.go`: reads new `VERSION`/`BUILD_DATE` env vars;
    stops reading `GIT_SHA` into a `revision` var passed to `httpserver.New`
    (replaced by the two new values).
  - `backend/internal/httpserver/httpserver.go`: `New`'s signature changes
    (`revision` param replaced by `version, buildDate`); `healthResponse`
    drops `Revision`; new `GET /api/about` handler + `aboutResponse` type.
  - `backend/internal/httpserver/httpserver_test.go`: existing revision
    assertion on `/health` updated to assert its absence; new test(s) for
    `/api/about`.
- **`Dockerfile`**: new `VERSION`/`BUILD_DATE` build-args and `ENV`s,
  alongside the existing `GIT_SHA` one.
- **`.github/workflows/nightly.yml`**: `publish` job computes and passes
  `VERSION`/`BUILD_DATE` as build-args; checkout step confirmed to fetch tag
  history.
- **`web/`**: new `AboutPage.tsx` + route; `AppLayout.tsx`'s `NAV_LINKS`
  gains an `/about` entry (last); `api/library.ts` gains a `getAbout()` call
  and its response type.
- **`mobile/`**: unchanged — no UI surface planned there for this.
- **`docs/`**: this backlog/TDR pair; `README.md`, `docs/deploy/synology.md`,
  `docs/ARCHITECTURE.md` (§3's `/health` description, new `/api/about` entry,
  decision-log row), and `deploy/docker-compose.yml`'s comment all updated to
  stop pointing at `/health` for build identity and point at the About
  page / `/api/about` instead.
- **Schema**: none.
