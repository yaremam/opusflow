# User Story: About Page & Build Versioning

## 1. User Value Statement

As a **self-hoster running opusflow**, I want to **see exactly which build is
running from within the app itself**, So that **I can tell whether an update
actually landed, and give an unambiguous version when reporting a bug**.

## 2. Strict Acceptance Criteria

### Versioning

- **AC-1**: The running build's version is derived from `git describe --tags
  --always` at image build time — the nearest tag plus commits-since and a
  short SHA when not built exactly on a tag (e.g. `v0.1.0-4-gabc123f`), or
  just the tag (e.g. `v0.1.0`) when it is.
- **AC-2**: An annotated git tag `v0.1.0` exists on `main` as of this
  feature, so every build going forward reports a real version rather than a
  bare commit hash.
- **AC-3**: The nightly CI workflow (`.github/workflows/nightly.yml`)
  computes the version string and a UTC build timestamp at build time and
  passes both into the image build, the same way it already does for
  `GIT_SHA` — no separate release step required for the nightly channel.
- **AC-4**: A build made without git tag metadata available (e.g. a shallow
  clone, or local `go run`) falls back to a fixed placeholder (`"dev"`)
  rather than failing the build or crashing the server.

### About page

- **AC-5**: A new "About" link appears as the last item in the top
  navigation, present on every page.
- **AC-6**: Visiting the About page shows: the app name, the version string
  (AC-1), the build's UTC timestamp, and a link to the project's GitHub
  repository.
- **AC-7**: The version and build timestamp shown on the page come from a new
  `GET /api/about` endpoint (`{"version", "buildDate"}`) — not hardcoded into
  the frontend build, so the same static web bundle reports correctly
  regardless of which backend build actually served it.

### `/health` contract change

- **AC-8**: `GET /health` no longer includes a `revision` field — it returns
  `{"status": "ok"}` only. Every place that documented checking `/health` for
  build identity (README, `docs/deploy/synology.md`,
  `deploy/docker-compose.yml`'s comments) is updated to point at the About
  page / `GET /api/about` instead.
