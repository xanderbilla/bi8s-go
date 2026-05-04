# Architecture

`bi8s-go` is a Go HTTP API that serves a media catalog (movies, people,
attributes) and orchestrates a video encoder pipeline. State lives in
DynamoDB and S3; the runtime is a single binary deployed to EC2 behind
NGINX, with full OpenTelemetry instrumentation.

## Component diagram

```
                 ┌──────────────────────────────────────────────┐
                 │                  Clients                     │
                 │   (web UI, mobile, internal admin tools)     │
                 └────────────────┬─────────────────────────────┘
                                  │ HTTPS
                                  ▼
                          ┌───────────────┐
                          │  NGINX (TLS)  │   prod only
                          └───────┬───────┘
                                  │ :8080
                                  ▼
              ┌────────────────────────────────────────────┐
              │        bi8s-go (chi router, middleware)    │
              │  ┌──────────────┬───────────────────────┐  │
              │  │ /v1/c/*      │ /v1/a/*               │  │
              │  │ consumer     │ admin (writes)        │  │
              │  └──────┬───────┴───────┬───────────────┘  │
              │         │               │                  │
              │  ┌──────▼───────┐ ┌─────▼────────────┐     │
              │  │ MovieService │ │ EncoderService    │    │
              │  │ PersonSvc    │ │  (ffmpeg → S3 HLS)│    │
              │  │ AttributeSvc │ │                   │    │
              │  └──────┬───────┘ └─────┬─────────────┘    │
              └─────────┼───────────────┼──────────────────┘
                        │               │
              ┌─────────▼──┐    ┌───────▼─────┐    ┌───────────┐
              │  DynamoDB  │    │     S3       │   │   Redis   │
              │ 4 tables   │    │ uploads+HLS  │   │ ratelimit │
              └────────────┘    └──────┬───────┘   └───────────┘
                                       │
                                       ├──── Loki backend
                                       └──── Tempo backend

  Telemetry path:
     bi8s-go ──OTLP/gRPC:4317──► otel-collector ──► Prometheus / Tempo / Loki
                                          ▲
                              promtail (docker logs) ┘
```

## Package layout (`app/`)

| Package                              | Responsibility                                                            |
| ------------------------------------ | ------------------------------------------------------------------------- |
| `cmd/api`                            | Process entry point, HTTP server, graceful shutdown.                      |
| `internal/app`                       | Config loading + validation, dependency wiring (`bootstrap.go`).          |
| `internal/http`                      | Router, middleware, handlers, parsers, validation, Swagger UI.            |
| `internal/http/middleware/ratelimit` | Pluggable memory/Redis rate-limit backends.                               |
| `internal/service`                   | Business logic (movie/person/attribute/encoder services).                 |
| `internal/repository`                | DynamoDB access layer (one file per table).                               |
| `internal/storage`                   | S3 upload/download/streaming helpers.                                     |
| `internal/encoder/queue`             | SQS queue publisher for encoder jobs.                                     |
| `internal/redis`                     | Redis client + rate-limit backend implementation.                         |
| `internal/aws`                       | AWS SDK config + client construction.                                     |
| `internal/observability`             | OTel tracer/meter/logger setup, HTTP metrics middleware.                  |
| `internal/logger`                    | `slog` setup (JSON in prod, text in dev).                                 |
| `internal/response`                  | The single response `Envelope` type used by every handler.                |
| `internal/errs`                      | Centralized `APIError` and constructors (`NewNotFound`, ...).             |
| `internal/validation`                | go-playground/validator setup + custom rules (`customdate`, `daterange`). |
| `internal/model`                     | Domain types (Movie, Person, Attribute, Encoder, Playback, etc.).         |
| `internal/env`                       | Typed env-var helpers (`GetInt`, `GetString`, `GetBool`).                 |
| `internal/ctxutil`                   | Request context helpers (request id, deadlines).                          |
| `internal/utils`                     | Cross-cutting helpers used by multiple packages.                          |

## Request lifecycle

For a typical `GET /v1/c/content/{contentId}`:

1. **NGINX** terminates TLS (prod) and forwards to the API on `:8080`.
2. **Middleware chain** (`router.go`):
   1. `cors.Handler` — origin allow-list, credentials enabled
   2. `RequestIDMiddleware` — sets `X-Request-ID`
   3. `HTTPMetrics.Middleware` — records `http_requests_total`, latency histogram
   4. `RequestLogger` — slog access log with status, duration, request id
   5. `middleware.Recoverer` — converts panics to 500 envelopes
   6. `SecureHeaders` — HSTS, X-Content-Type-Options, frame deny, etc.
   7. `MaxBytesJSON` — caps request body size (default 1 MiB)
   8. **Global rate limit** (memory or Redis backend)
   9. **Per-route rate limit** for write endpoints (encoder/movie/person)
   10. `middleware.Timeout` — per-request deadline (default 60 s)
3. **Route handler** parses + validates the request, calls the service.
4. **Service** invokes one or more **repositories** and returns a domain object.
5. **Handler** writes a `response.Envelope` via `response.Success(...)`.
6. **otelhttp** wraps the entire handler so the route pattern is the span name.

## Cross-cutting invariants

- **Single envelope shape**: every successful or failed response uses
  `response.Envelope` (see [API.md](API.md)).
- **Centralized errors**: handlers never write `http.Error` directly — they
  return `errs.APIError` values via `errs.Write(w, r, err)`.
- **No global state**: all dependencies are constructed in
  `app.Bootstrap` and injected through `*app.Application`.
- **Graceful shutdown**: SIGINT/SIGTERM triggers an HTTP server shutdown,
  closes rate-limit backends, drains in-flight encoder jobs (up to 120 s),
  and flushes OTel exporters.

## External services

| Service                   | Purpose                                      | Local equivalent                   |
| ------------------------- | -------------------------------------------- | ---------------------------------- |
| AWS DynamoDB              | Movie / Person / Attribute / Encoder tables  | DynamoDB Local container           |
| AWS S3                    | Uploads, HLS output, Loki/Tempo blob backend | MinIO                              |
| AWS SQS _(optional)_      | Encoder job queue                            | (in-memory worker pool by default) |
| Redis                     | Shared rate-limit state                      | redis container                    |
| OTel Collector            | Telemetry router                             | otel-collector container           |
| Tempo / Prometheus / Loki | Trace / metric / log storage                 | local containers                   |
| Grafana                   | Dashboards + alerts                          | local container                    |
