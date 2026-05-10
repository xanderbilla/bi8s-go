# syntax=docker/dockerfile:1.7

# Base stage shared by build and quality checks.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS base

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Install CA certificates so HTTPS calls work correctly in runtime image.
RUN apk add --no-cache ca-certificates git

# Download dependencies first to maximize Docker layer cache reuse.
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build a static binary.
COPY cmd ./cmd
COPY internal ./internal

# Quality stage: executes recommended checks in a reproducible container.
# Tool versions are pinned to keep the build deterministic and to avoid
# pulling unreviewed transitive supply-chain changes from upstream HEAD.
FROM base AS quality

ENV PATH="/go/bin:${PATH}"

ARG GOLANGCI_LINT_VERSION=v1.61.0
ARG STATICCHECK_VERSION=2024.1.1
ARG GOVULNCHECK_VERSION=v1.1.3
ARG GOSEC_VERSION=v2.21.4

RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION} \
    && go install honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION} \
    && go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION} \
    && go install github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}

RUN go fmt ./... \
    && test -z "$(gofmt -l .)" \
    && go vet ./... \
    && go test ./... \
    && golangci-lint run ./... \
    && staticcheck ./... \
    && govulncheck ./... \
    && gosec -quiet ./...

# Build stage: compiles the Go API binary for the target OS/architecture.
FROM base AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags="-s -w \
    -X github.com/xanderbilla/bi8s-go/internal/buildinfo.Version=${VERSION} \
    -X github.com/xanderbilla/bi8s-go/internal/buildinfo.Commit=${COMMIT} \
    -X github.com/xanderbilla/bi8s-go/internal/buildinfo.Date=${BUILD_DATE}" \
    -o /out/api ./cmd/api

# Runtime stage: minimal Debian image with ffmpeg/ffprobe for encoder health checks.
FROM debian:bookworm-slim AS runtime

ARG APP_UID=10001
ARG APP_GID=10001
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# OCI image labels — surfaced by `docker inspect` and ECR/registry UIs.
LABEL org.opencontainers.image.title="bi8s-go" \
    org.opencontainers.image.description="bi8s-go API (EnternFlix backend)" \
    org.opencontainers.image.source="https://github.com/xanderbilla/bi8s-go" \
    org.opencontainers.image.licenses="MIT" \
    org.opencontainers.image.vendor="xanderbilla" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.revision="${COMMIT}" \
    org.opencontainers.image.created="${BUILD_DATE}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid ${APP_GID} app \
    && useradd  --system --uid ${APP_UID} --gid ${APP_GID} --no-create-home --shell /usr/sbin/nologin app

WORKDIR /app

COPY --from=builder /out/api /app/api

USER app:app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/v1/livez >/dev/null 2>&1 || exit 1

# Start the API server.
ENTRYPOINT ["/app/api"]
