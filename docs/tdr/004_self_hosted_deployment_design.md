# TDR 004: Self-Hosted Deployment via Prebuilt Image

## 1. Context & Architectural Requirements

Per `docs/ARCHITECTURE.md` §1, opusflow already ships as one Docker image
(root `Dockerfile`) with a root `docker-compose.yml` that builds that image
from source and runs it alongside Postgres. That stack is a good local
dev/build loop, but it requires cloning the repo and running `docker compose
up -d --build` on the target machine — on a NAS with no Go/Node toolchain and
limited CPU (the intended first target: a Synology), a from-source build is
slow and pulls in build-time tooling (`node:22-alpine`, `golang:1.24`) that
never needs to touch the runtime machine at all.

The `docuflow` project (a sibling project by the same author) already solved
this exact problem — feature 021 there publishes a nightly prebuilt image to
GHCR and ships a separate `deploy/docker-compose.yml` that only pulls and
runs it. This TDR adopts that same shape for opusflow, adapted for two
differences from docuflow: opusflow has no blob-storage service (MinIO) to
bundle, and opusflow's core job — scanning a real, arbitrarily-organized
music library off host disk — has no docuflow equivalent, since docuflow's
uploads go through the app itself into MinIO rather than being scanned off a
mounted host directory.

## 2. Alternatives Evaluated

### Alternative: build platform — amd64 only vs. amd64+arm64

- **amd64 only (docuflow's choice)** — Pros: single-arch build, faster CI,
  matches docuflow's known deploy target (a specific Celeron NAS) exactly.
  Cons: opusflow's deploy target isn't pinned to one known machine the way
  docuflow's is — the household may run it on an ARM-based Synology, a
  Raspberry Pi, or an Apple Silicon VM later, and an amd64-only image simply
  fails to run there with no obvious error.
- **amd64+arm64 (chosen)** — Pros: one image reference works everywhere
  regardless of which machine ends up running it, now or later — no
  per-install "check your CPU first" step. Cons: multi-platform `buildx`
  roughly doubles publish-job build time versus a single-arch build; accepted
  since this runs on a low-frequency nightly schedule (AC-2 also skips it
  entirely on unchanged commits), not on every push.

### Alternative: Postgres in the deploy stack — bundled vs. external

- **External Postgres** — Pros: leaner deploy stack, no second stateful
  container for opusflow itself to manage. Cons: pushes the actual hardest
  part of self-hosting (getting a reachable, correctly-authenticated
  Postgres) onto the user with zero guidance; contradicts the "pull and run"
  simplicity this feature exists to deliver.
- **Bundled Postgres (chosen)** — Pros: matches docuflow's proven pattern —
  `docker compose up -d` is genuinely all that's needed, healthcheck-gated so
  the app container doesn't race Postgres's startup. Cons: the self-hoster
  now owns that Postgres container's backups/upgrades themselves, same
  tradeoff docuflow already accepted and documented (bind-mount option in a
  comment for anyone who wants DSM to manage the volume directly).

### Alternative: music library mounting — single root vs. multiple roots

- **Single bind-mount, one `LIBRARY_ROOTS` entry** — Pros: simplest possible
  compose file. Cons: real households split music across more than one
  share/volume in practice (e.g. a shared family library plus a personal
  one) — `library.ParseRoots` (backend/internal/library/roots.go) already
  supports a comma-separated list of roots specifically for this;
  restricting the deploy stack to one root would under-use a capability the
  backend already has.
- **Multiple bind-mounts / roots (chosen)** — Pros: `deploy/docker-compose.yml`
  mounts one `volumes:` line per host music folder to its own container path
  (`/music/<name>`), and `LIBRARY_ROOTS` lists all of them — directly matches
  how `ParseRoots` already works, no backend change needed. Cons: compose has
  no native "N host paths from one setting" mechanism, so adding a folder
  means editing a `volumes:` line and the comma-separated `LIBRARY_ROOTS`
  value together; documented as a copy-and-extend pattern directly in
  `docker-compose.yml`'s comments and in `docs/deploy/synology.md` rather
  than solved generically.

### Alternative: configuration mechanism — `.env` file vs. inline values in `docker-compose.yml`

- **`.env` file (docuflow's approach, initially adopted here too)** — Pros:
  one place for every setting, separate from the file that defines the
  stack's shape; matches Docker Compose's own convention and docuflow's
  proven `deploy/.env.example`. Cons: for opusflow's actual first deploy
  target, DSM Container Manager's simplest project-creation path is pasting
  a single `docker-compose.yml`'s contents directly into an inline text
  editor — a second file that also needs uploading (or a File Station
  round-trip to create it alongside the first) adds a step that path doesn't
  need, and it's easy to paste the compose file into that inline editor
  while forgetting `.env` never travels with it.
- **Inline literal values directly in `docker-compose.yml`, marked "EDIT
  THIS" (chosen)** — Pros: exactly one file to paste, copy, or hand to
  someone — matches Container Manager's inline-paste path with no extra
  step; every value a self-hoster might touch (music folder path, host
  port, database password) is a comment-marked literal right where it's
  used. Cons: no single source of truth for a value used in two places — the
  Postgres password appears both in `POSTGRES_PASSWORD` and embedded in
  `DATABASE_URL`, and both must be edited together (called out explicitly in
  a comment on each); `${VAR}` substitution with defaults would have avoided
  that duplication but reintroduces the "is there a `.env` I'm missing"
  question this alternative exists to remove.

### Alternative: publish cadence — nightly cron vs. on every push to main

- **On every push to main** — Pros: an update lands on the registry the
  moment it's merged. Cons: spends CI minutes on every merge regardless of
  whether anyone's about to pull; no natural batching point.
- **Nightly cron + manual `workflow_dispatch` (chosen)** — Pros: matches
  docuflow's proven pattern exactly, including the skip-if-unchanged check
  (`skopeo inspect` the published image's `org.opencontainers.image.revision`
  label against `$GITHUB_SHA` before doing any build work) so an idle `main`
  costs nothing; `workflow_dispatch` covers "I want today's build right now."
  Cons: a merge to `main` isn't reachable on the registry until the next
  nightly run (or a manual dispatch) — acceptable for a nightly-quality
  channel that explicitly excludes `:latest`.

## 3. Structural Decision

**CI**: new `.github/workflows/nightly.yml`, structurally identical to
docuflow's — a `changes` job (skip-check via `skopeo`), a `test` job (`go
test ./...` plus `pnpm --filter web build` and `pnpm --filter web lint`,
gating the next job), and a `publish` job (`docker/build-push-action`,
`platforms: linux/amd64,linux/arm64`, tags `:nightly` /
`:nightly-<date>` / `:sha-<short>`, pushed to `ghcr.io/${{ github.repository
}}` using the default `GITHUB_TOKEN`).

**Dockerfile**: gains a `GIT_SHA` build-arg (default `dev`), stamped as both
an `ENV` (read by the running process) and OCI labels
(`org.opencontainers.image.source` / `.description` / `.revision`) — the
`revision` label is what the nightly workflow's skip-check reads back.
`GET /health`'s response gains a `revision` field sourced from that env var,
so a deployed instance can be identified without pulling registry metadata.

**`deploy/`**: a new top-level directory, sibling to (not a replacement for)
the root `docker-compose.yml`. `deploy/docker-compose.yml` is the single
file a self-hoster needs — no `deploy/.env`. It runs two services — `app`
(the published image, `restart: unless-stopped`) and `postgres`
(`postgres:17-alpine`, healthcheck-gated, named volume with a commented DSM
bind-mount alternative) — plus one `volumes:` entry per configured music
folder and one for `ARTWORK_DIR`. Every self-hoster-facing value (music
folder host path, host port, Postgres password) is a literal directly in the
file, marked with an "EDIT THIS" comment at the point of use.

**Docs**: `docs/deploy/synology.md` is the concrete, numbered walkthrough for
DSM's Container Manager — pasting `docker-compose.yml`'s contents into
Container Manager's inline project editor, editing the marked lines there
(port, folder mapping, password), and a `/health` verification step — linked
from a new README "Self-hosting" section.

## 4. Cross-Workspace Implications

- **`backend/`**: `httpserver.healthResponse` gains a `revision` field;
  `cmd/server/main.go` reads `GIT_SHA` (defaulting to `"dev"` when unset, the
  same convention `PORT`/`STATIC_DIR`/`ARTWORK_DIR` already use for
  "unset means local/dev, not an error"). No schema change, no new package.
- **`Dockerfile`**: `ARG GIT_SHA=dev`, `ENV GIT_SHA=${GIT_SHA}`, OCI labels on
  the runtime stage — additive, doesn't change any existing build stage.
- **`.github/`**: first workflow in the repo — `nightly.yml` only; no
  separate PR-triggered CI is added by this feature (matches docuflow, which
  also has exactly one workflow file).
- **`deploy/`** (new): a single `docker-compose.yml`, no `.env`, independent
  of the root compose file used for local dev/build.
- **`docs/`**: this backlog/TDR pair; `docs/deploy/synology.md` (new);
  `README.md` gains a "Self-hosting" section; `docs/ARCHITECTURE.md` §3
  (health endpoint's `revision` field) and §5 (decision-log row) updated once
  implementation lands.
- **`web/`, `mobile/`**: unchanged — this feature is packaging/ops only, no
  UI surface.
