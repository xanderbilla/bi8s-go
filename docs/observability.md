# Observability

The service ships a lean stdlib-only observability stack — no Prometheus,
no OTel SDK, no third-party log shippers in the binary. Everything below is
emitted by the Go standard library.

## Metrics (`expvar`)

Metrics are exposed exclusively via the standard library `expvar` package.
There is **no** Prometheus exporter, scrape endpoint, or OpenMetrics format —
production monitoring is expected to read JSON via `expvar` or scrape the
Debug endpoint described below.

- `GET /v1/debug/vars` exposes the full `expvar` JSON snapshot.
- The endpoint is **gated to non-production environments only**
  (`APP_ENV != "prod"`). In production the route is not registered, returning
  404 if probed.
- Counters live in [internal/http/metrics.go](app/internal/http/metrics.go)
  and cover request totals, per-status counts, and per-route latency buckets.

## Structured logging (`log/slog`)

- All log lines are emitted via `slog.Default()` with a JSON handler configured
  in `internal/logger`.
- Every request handler runs inside `RequestLogger`
  ([internal/http/middleware.go](app/internal/http/middleware.go)), which emits
  one `http_request` line per request with these attributes:

  | key           | description                                                                             |
  | ------------- | --------------------------------------------------------------------------------------- |
  | `request_id`  | Chi request id, also returned as `X-Request-ID`                                         |
  | `method`      | HTTP method                                                                             |
  | `path`        | URL path (no query string)                                                              |
  | `status`      | HTTP status code (defaults to 200 if not set)                                           |
  | `bytes`       | Response body bytes written                                                             |
  | `duration_ms` | Wall time in milliseconds                                                               |
  | `remote_addr` | Best-effort client IP (resolved via `ratelimit.GetClientIP` honoring `TRUSTED_PROXIES`) |

  Level mapping: `>=500 → Error`, `>=400 → Warn`, otherwise `Info`.

- Liveness (`/v1/livez`) and readiness (`/v1/readyz`) probes are skipped at
  Info level to keep production logs quiet; enable `slog.LevelDebug` to see
  them.

- The `http.Server.ErrorLog` is wired to `slog` so TLS / accept errors land in
  the same JSON stream rather than going to stderr unstructured.

## Request correlation

- A custom `RequestID` middleware (in
  [internal/http/middleware.go](app/internal/http/middleware.go)) reads or
  generates a UUIDv4 request id, stores it in the chi request context (so
  `chi.GetReqID` keeps working), copies it into the application context via
  `internal/ctxutil`, and sets the `X-Request-ID` response header so callers
  can correlate logs across services.
- The id appears in:
  - every access log line (`request_id` attribute),
  - every error envelope (`request_id` field in the JSON response),
  - every panic recovery line (via chi's `Recoverer`).

## Health probes

- `GET /v1/livez` — process is up. Cheap, returns 200 immediately.
- `GET /v1/readyz` — process is ready to serve traffic. Returns 503 while the
  readiness flag is false (during startup before `transport.SetReady(true)` and
  after shutdown signal once `transport.SetReady(false)` is called).
- `GET /v1/health` — deep health check; runs every dependency probe in
  parallel with a 2-second context and returns `{checks: {name: up|down}}`.

Probes are wired in [cmd/api/main.go](app/cmd/api/main.go); `SetReady(true)`
is called inside the listener goroutine just before `srv.ListenAndServe`, and
`SetReady(false)` is called immediately before `srv.Shutdown`.

## Response envelope

Every response — success or error — uses the canonical envelope defined in
[internal/response/response.go](app/internal/response/response.go):

```json
{
  "success": true,
  "status": 200,
  "code": "",
  "message": "ok",
  "data": {},
  "details": null,
  "path": "/v1/c/content",
  "request_id": "abc-123",
  "timestamp": "2025-01-01T00:00:00Z"
}
```

The envelope intentionally omits a separate top-level `error` field — error
responses surface the human-readable text in `message` and the stable
machine-readable identifier in `code` (see `docs/errors.md`).

There is exactly one success writer (`response.Success`) and one error writer
(`response.Error`). Error envelopes from `errs.Write` use the same shape with
`success: false`, a stable machine-readable `code`, and an optional `details`
payload (used for validation errors).
