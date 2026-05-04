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
FROM base AS quality

ENV PATH="/go/bin:${PATH}"

RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest \
    && go install honnef.co/go/tools/cmd/staticcheck@latest \
    && go install golang.org/x/vuln/cmd/govulncheck@latest

RUN go fmt ./... \
    && test -z "$(gofmt -l .)" \
    && go vet ./... \
    && go test ./... \
    && golangci-lint run ./... \
    && staticcheck ./... \
    && govulncheck ./...

# Build stage: compiles the Go API binary for the target OS/architecture.
FROM base AS builder

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage: minimal Debian image with ffmpeg/ffprobe for encoder health checks.
FROM debian:bookworm-slim AS runtime

ARG APP_UID=10001
ARG APP_GID=10001

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates ffmpeg wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid ${APP_GID} app \
    && useradd  --system --uid ${APP_UID} --gid ${APP_GID} --no-create-home --shell /usr/sbin/nologin app

WORKDIR /app

COPY --from=builder /out/api /app/api

USER app:app

EXPOSE 8080

# Start the API server.
ENTRYPOINT ["/app/api"]
