# opusflow

A private, self-hosted music platform — an alternative to Roon that avoids
proprietary protocols wherever that promise is achievable. Unifies a local
music library with connected streaming accounts (Spotify, Apple Music) into
one library, plays music anywhere in the house, and helps you discover and
keep up with the artists you care about. See [`docs/vision.md`](docs/vision.md)
for the full product vision, and [`docs/backlog/`](docs/backlog/) for what's
actually built so far.

## Repository layout

- `backend/` — Go backend (`cmd/server`, `internal/httpserver`)
- `web/` — React web app (Vite + TypeScript)
- `mobile/` — React Native mobile app, Android + eventually iOS (Expo + TypeScript)
- `deploy/` — self-hosting stack: pulls the prebuilt image instead of
  building from source (see "Self-hosting" below)
- `docs/vision.md` — product vision. Start here.
- `docs/ARCHITECTURE.md` — living system architecture.
- `docs/backlog/` — user stories and acceptance criteria, one per feature.
- `docs/tdr/` — Technical Design Records, one per feature.
- `docs/deploy/` — self-hosting walkthroughs for specific platforms.

The backend and web app are built and shipped together as **one Docker image**
(root `Dockerfile`) — the Go binary serves both the API and the built web
app. The mobile app is a separate native build, not part of this image.

## Prerequisites

- Docker + Docker Compose

That's it for running the app — no local Go/Node toolchain required.

## Run it

```sh
cp .env.example .env
docker compose up -d --build
```

This builds the app image (web build + Go build, multi-stage), and starts
Postgres and the app together.

Verify it's up:
```sh
curl http://localhost:8090/health
```
Then visit `http://localhost:8090` in a browser for the web app.

If port `8090` (or `5432` for Postgres) is already taken on your machine,
override the host-side port mapping in `docker-compose.yml`.

To rebuild after changing the code:
```sh
docker compose up -d --build app
```

## Self-hosting (prebuilt image)

For running opusflow on a NAS or other machine without a Go/Node toolchain,
[`deploy/docker-compose.yml`](deploy/docker-compose.yml) pulls the nightly
image published to `ghcr.io/yaremam/opusflow` instead of building from
source. It's a single self-contained file — no `.env` to manage — just edit
the lines marked "EDIT THIS" (your music folder's host path, the app port,
and the Postgres password) directly in it, then:

```sh
cd deploy
docker compose pull
docker compose up -d
```

See [`docs/deploy/synology.md`](docs/deploy/synology.md) for a full
step-by-step walkthrough on Synology DSM's Container Manager, including
mapping ports, mapping your music library, and updating to newer builds.

## Developing without Docker

Faster iteration loop for editing code directly.

### Backend

Requires Go 1.24+.
```sh
cd backend
go run ./cmd/server        # PORT defaults to 8080; STATIC_DIR unset = API only
go test ./...
```

### Web

Requires Node 20+ and pnpm (`corepack enable && corepack use pnpm@9` — pnpm
11+ needs Node 22).
```sh
pnpm install
pnpm --filter web dev
```

### Mobile

```sh
pnpm --filter mobile start
```
Scan the QR code with the Expo Go app on your phone, or press `a`/`i` for an
Android/iOS simulator (iOS requires macOS + Xcode).

## Database persistence

Postgres's data directory is mounted to a named Docker volume (`pgdata` in
`docker-compose.yml`), so your data survives `docker compose stop`/`up -d`,
`restart`, and `down` (without `-v`). It's only wiped by:
```sh
docker compose down -v
```
