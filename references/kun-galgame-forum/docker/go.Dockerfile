#
# Parametric build for kungal's PURE-GO binaries: the API server (cmd/server)
# and every one-off cmd/migrate / sync-moemoepoint / backfill-* tool.
#
#   docker build -f docker/go.Dockerfile --build-arg CMD=server  -t kungal-api .
#   docker build -f docker/go.Dockerfile --build-arg CMD=migrate -t kungal-migrate .
#
# kungal imports NO cgo (image processing is image_service's job, called over
# HTTP), so everything is a CGO_ENABLED=0 static binary on distroless — there
# is no cgo.Dockerfile here (unlike oauth/image, which embed libwebp).
#
# Build context MUST be the repo root.
ARG GO_VERSION=1.26

# ---- build ----
FROM golang:${GO_VERSION}-trixie AS build
WORKDIR /src
# Manifests first → module-download layer is cached until go.mod/sum change.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download
COPY apps/api/ ./
ARG CMD=server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /out/app ./cmd/${CMD}

# ---- run ----
# distroless/static: ~2MB base, no shell, nonroot (uid 65532). Bundles
# ca-certificates (outbound HTTPS: OAuth, image_service, wiki, B2, SMTP TLS)
# + tzdata (the daily reset cron / admin stats pin Asia/Shanghai).
FROM gcr.io/distroless/static-debian13:nonroot
# distroless :nonroot defaults WORKDIR to /home/nonroot (uid 65532's home), NOT
# /. cmd/migrate's -path defaults to "migrations" (relative to CWD), so without
# pinning CWD it looked in /home/nonroot/migrations, found nothing, and silently
# "ran" zero migrations (filepath.Glob on a missing dir → empty, no error). Pin
# WORKDIR to / so -path "migrations" resolves to the /migrations copied below.
WORKDIR /
COPY --from=build /out/app /app
# cmd/migrate reads SQL from disk at runtime (-path "migrations" → /migrations,
# given WORKDIR / above). Harmless dead weight in the api/server image.
COPY apps/api/migrations /migrations
USER nonroot:nonroot
ENTRYPOINT ["/app"]
# The container HEALTHCHECK is set in docker-compose.yml as:
#   test: ["CMD", "/app", "healthcheck"]   # → GETs /healthz, exits 0/1
