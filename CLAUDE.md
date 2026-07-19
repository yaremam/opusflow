# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

opusflow is a full-stack platform: a Go backend, a React web app, and a React Native mobile app (Android, eventually iOS), packaged together into a Docker container. It's a monorepo — backend, web, and mobile live as workspaces in this single repository (`backend/`, `web/`, `mobile/`).

**Status**: workspaces are scaffolded (default starter templates + a health
endpoint), no product features are built yet. See
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the current system shape.

## Stack

- Backend: Go 1.24, stdlib `net/http` (`backend/`)
- Web frontend: React 19 + Vite + TypeScript (`web/`)
- Mobile: Expo (React Native) + TypeScript (`mobile/`) — chosen specifically to share component patterns and logic with the web frontend
- Database: PostgreSQL 17 (declared in `docker-compose.yml`, not yet wired into backend code)
- Package manager (web/mobile): pnpm workspaces, pinned to `pnpm@9` via corepack — `pnpm@11`+ requires Node 22, this environment has Node 20
- Packaging: backend + web ship together as **one** Docker image (root `Dockerfile`, multi-stage); the Go binary serves both the API and the built web static files. Mobile is a separate native build, not part of this image.

## Commands

```sh
# Full stack via Docker (see README.md)
docker compose up -d --build

# Backend only
cd backend && go run ./cmd/server   # go test ./...

# Web only
pnpm --filter web dev               # pnpm --filter web build

# Mobile only
pnpm --filter mobile start
```

## Conventions

- Commit messages: Conventional Commits (`feat:`, `fix:`, `chore:`, etc.)
- Branches: `feature/<name>`, `fix/<name>`

## Feature development process

Every feature starts with the `/new-feature` skill (`.claude/skills/new-feature/`),
which: grills the idea first (`grilling` skill) to sharpen it before anything is
written down, mocks up and gets sign-off on any new UI screen (an Artifact) before
component/handler code is written, then scaffolds a matched pair of docs:

- `docs/backlog/NNN_<slug>.md` — user story: value statement + strict acceptance criteria.
- `docs/tdr/NNN_<slug>_design.md` — design doc: context, alternatives evaluated, structural decision, cross-workspace implications.

Both are numbered together (shared `NNN` counter across both directories). See
[`docs/README.md`](docs/README.md) for the full layout and
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the living system architecture
— update it whenever a feature changes a component boundary, adds a table, or
reverses an earlier decision.

## Implementation

- **TDD**: every implementation starts with a failing test defining the behavior
  (red-green-refactor), driven from the backlog entry's acceptance criteria.
- **UI mockups**: any new web or mobile screen gets a mockup (an Artifact) and
  explicit sign-off before implementation — see the `/new-feature` skill. Minor
  tweaks to an existing screen don't need this.
