# --- build web ---
FROM node:22-alpine AS web-build
WORKDIR /app
RUN corepack enable
COPY pnpm-workspace.yaml package.json pnpm-lock.yaml ./
COPY web/package.json web/package.json
RUN pnpm install --filter web --frozen-lockfile
COPY web ./web
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
EXPOSE 8080
ENTRYPOINT ["/app/server"]
