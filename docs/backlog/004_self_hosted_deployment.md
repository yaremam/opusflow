# User Story: Self-Hosted Deployment via Prebuilt Image

## 1. User Value Statement

As a **household member self-hosting opusflow on a NAS (e.g. Synology)**, I
want to **pull a prebuilt image from a registry and start the app with
Docker Compose**, So that **I can run opusflow without cloning the repo or
installing a Go/Node toolchain, and follow a concrete walkthrough for
mapping my music folders and ports**.

## 2. Strict Acceptance Criteria

### Image publishing

- **AC-1**: A CI workflow builds and publishes a multi-platform
  (`linux/amd64` + `linux/arm64`) opusflow image to `ghcr.io/yaremam/opusflow`
  on a nightly schedule (and on manual dispatch), tagged `:nightly`,
  `:nightly-<date>`, and `:sha-<short>`. `:latest` is never tagged by this
  workflow — reserved for a future real-release process.
- **AC-2**: The workflow skips the build entirely when `main` hasn't moved
  since the last published `:nightly` (compared via the
  `org.opencontainers.image.revision` OCI label on the published image), so
  an unchanged `main` doesn't burn CI minutes or publish a redundant image.
- **AC-3**: No image is published unless, on that exact commit, the backend
  test suite (`go test ./...`) passes and the web app builds and lints
  cleanly (`pnpm --filter web build`, `pnpm --filter web lint`).
- **AC-4**: `GET /health` reports the git revision the running image was
  built from, so a self-hoster can confirm which nightly is actually
  deployed.

### Self-hosting stack

- **AC-5**: A `deploy/` directory provides a single self-contained
  `docker-compose.yml` that pulls the prebuilt `ghcr.io/yaremam/opusflow:nightly`
  image (no `build:`) and runs Postgres alongside it. No separate `.env`
  file is required — every value a self-hoster might need to change (music
  folder path, host port, database password) is a literal, clearly marked
  line directly in that one file, so it can be pasted as-is into tools like
  Synology Container Manager's inline compose editor. This is separate from
  the repo-root `docker-compose.yml`, which remains the local
  build-from-source dev stack and is unchanged by this feature.
- **AC-6**: The deploy stack supports mounting more than one host music
  folder (matching `LIBRARY_ROOTS`' existing comma-separated-paths design),
  documented with a copy-and-extend pattern for adding additional folders.
- **AC-7**: The deploy stack persists `ARTWORK_DIR` and the Postgres data
  directory across restarts via named volumes, with the DSM bind-mount
  equivalent shown in a comment for each.
- **AC-8**: Starting the stack with `docker compose pull && docker compose up
  -d` after only editing the marked lines in `docker-compose.yml` itself
  (no other file) results in a working app reachable on the configured host
  port, with `/health` returning `ok`.

### Documentation

- **AC-9**: `docs/deploy/synology.md` walks a user through self-hosting on a
  Synology NAS using DSM's Container Manager: creating the project by
  pasting `docker-compose.yml`'s contents directly into Container Manager's
  inline editor, editing the marked lines there (host port, music folder
  path(s), database password), and verifying the app is up via `/health`.
- **AC-10**: `README.md` links to `deploy/` and `docs/deploy/synology.md`
  from a new "Self-hosting" section, distinct from the existing "Run it"
  (local dev/build) instructions.
