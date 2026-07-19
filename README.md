# opusflow

_Feature description to be filled in as features land — see
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and [`docs/backlog/`](docs/backlog/)
for what's planned and built so far._

## Repository layout

- `backend/` — Go backend (`cmd/server`, `internal/httpserver`)
- `web/` — React web app (Vite + TypeScript)
- `mobile/` — React Native mobile app, Android + eventually iOS (Expo + TypeScript)
- `docs/ARCHITECTURE.md` — living system architecture. Start here.
- `docs/backlog/` — user stories and acceptance criteria, one per feature.
- `docs/tdr/` — Technical Design Records, one per feature.

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
