# --- build web ---
FROM node:24-alpine AS web-build
WORKDIR /app
RUN corepack enable
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml ./
COPY web/package.json web/package.json
COPY packages/player-core/package.json packages/player-core/package.json
RUN pnpm install --filter web --frozen-lockfile
COPY web ./web
COPY packages ./packages
RUN pnpm --filter web build

# --- build backend ---
FROM golang:1.24 AS backend-build
WORKDIR /app
COPY backend/go.mod ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server ./cmd/server

# --- runtime ---
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=backend-build /out/server ./server
COPY --from=web-build /app/web/dist ./web
ENV STATIC_DIR=/app/web

# Build revision, stamped by the nightly pipeline (TDR 004). Kept as an OCI
# label — the nightly workflow reads `revision` back off the published image
# to decide whether a new commit exists to build. Declared this late so the
# varying ARGs only bust this final metadata layer, never the compile or
# copy layers.
ARG GIT_SHA=dev
LABEL org.opencontainers.image.source="https://github.com/yaremam/opusflow" \
      org.opencontainers.image.description="opusflow — self-hosted music platform" \
      org.opencontainers.image.revision="${GIT_SHA}"

# App version + build timestamp (TDR 009), surfaced by `GET /api/about` for
# the About page. VERSION is `git describe --tags --always` at build time
# (e.g. "v0.1.0-4-gabc123f"); BUILD_DATE is a UTC ISO-8601 timestamp. Both
# default to values that make sense for an unstamped local build.
ARG VERSION=dev
ARG BUILD_DATE=
ENV VERSION=${VERSION}
ENV BUILD_DATE=${BUILD_DATE}

EXPOSE 8080
ENTRYPOINT ["/app/server"]
