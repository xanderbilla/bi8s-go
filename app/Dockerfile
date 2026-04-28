# syntax=docker/dockerfile:1.7

# Build stage: compiles the Go API binary for the target OS/architecture.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Install CA certificates so HTTPS calls work correctly in runtime image.
RUN apk add --no-cache ca-certificates

# Download dependencies first to maximize Docker layer cache reuse.
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build a static binary.
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Runtime stage: minimal, non-root image for security.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/api /app/api

EXPOSE 8080

# Start the API server.
ENTRYPOINT ["/app/api"]
