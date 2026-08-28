# Build context is the repo root, not server/, because the image also carries
# migrations/ so Fly's release_command can apply them before a new version
# takes traffic.

# ---------------------------------------------------------------- build
FROM golang:1.27-alpine AS build
WORKDIR /src

# Dependencies first, so a source-only change reuses the module cache.
COPY server/go.mod server/go.sum ./server/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd server && go mod download

COPY server ./server

# CGO off gives a static binary, which is what lets the final stage be
# distroless. Trimpath and -w -s drop build paths and debug info.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd server && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/migrate ./cmd/migrate

# ---------------------------------------------------------------- runtime
# Distroless: no shell, no package manager, nothing to pivot to. The API is
# reachable from the public internet and takes a bearer secret (§6.5), so the
# smaller the surface the better.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY migrations /app/migrations

USER nonroot:nonroot
EXPOSE 8080

# CMD, not ENTRYPOINT. Fly APPENDS release_command to a container's entrypoint,
# so with ENTRYPOINT ["/app/api"] the release step runs
#   /app/api /app/migrate -dir /app/migrations up
# -- the API binary with stray arguments, which ignores them, boots, and fails.
# The migration never runs, and the error looks like a database problem.
CMD ["/app/api"]
