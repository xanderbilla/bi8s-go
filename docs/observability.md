# Observability

`bi8s-go` ships first-class **logs, metrics, and traces** — the three
pillars are wired through a single OTel collector and visualized in a
local Grafana stack. All observability config lives under
`observability/`.

## Pipeline

```
                       ┌──────────────────────────┐
        OTLP gRPC      │                          │
   ┌────────────────► │                          │ ── traces ──► Tempo (S3)
   │                   │      otel-collector      │
   │                   │   (observability/        │ ── metrics ──► Prometheus :8889
bi8s-go ───────────►   │   otel-collector-config) │
   │                   │                          │ ── logs ────►  (none — see below)
   │                   └──────────────────────────┘
   │
   └─ slog JSON to stdout ──► Promtail ──► Loki ──► (S3 chunk store)

                                                 ┌────────────┐
                                  All three ───► │  Grafana   │
                                                 └────────────┘
```

## Logs

- Library: `log/slog` with JSON handler in `prod`, text handler in `dev`.
- Configured by `internal/logger`. Level via `LOG_LEVEL`
  (`debug|info|warn|error`).
- Fields always present: `time`, `level`, `msg`, `service.name`,
  `request_id` (when in HTTP scope), `trace_id`, `span_id`.
- Container stdout → Docker driver → Promtail
  (`observability/promtail-config.yaml`) → Loki
  (`observability/loki-config.yaml`).
- Loki uses S3 (`bi8s-storage-*/loki/`) as the chunk store with
  boltdb-shipper indexes.

### What gets logged

| Event               | Level   | Notes                                                |
| ------------------- | ------- | ---------------------------------------------------- |
| HTTP request finish | `info`  | method, path, status, latency_ms, request_id, ua     |
| Validation failure  | `info`  | field, rule, value redacted if sensitive             |
| 5xx                 | `error` | full error chain, no stack (Go stack only on panics) |
| Recoverer panic     | `error` | stack trace                                          |
| Bootstrap           | `info`  | versions, config summary (no secrets)                |

## Metrics

- Exposed via OTel SDK → collector OTLP receiver → Prometheus exporter
  on `:8889`.
- Prometheus scrapes the collector (`observability/prometheus.yml`).
- Recording + alerting rules in `observability/prometheus-rules.yml`.

### Built-in series

| Metric                                       | Type            | Notes                                    |
| -------------------------------------------- | --------------- | ---------------------------------------- |
| `http_server_request_duration_seconds`       | histogram       | path, method, status.                    |
| `http_server_active_requests`                | gauge           | concurrency.                             |
| `http_server_request_body_size_bytes`        | histogram       |                                          |
| `process_*` / `go_*`                         | gauges/counters | runtime metrics from OTel host detector. |
| `bi8s_encoder_jobs_total{status}`            | counter         | encoder outcomes.                        |
| `bi8s_encoder_job_duration_seconds`          | histogram       | per-job time.                            |
| `bi8s_dynamodb_calls_total{op,table,status}` | counter         | repository instrumentation.              |

### Alerts

`observability/prometheus-rules.yml` ships with starter alerts:

- API 5xx rate > 1% for 5 min.
- p95 latency > 1.5 s for 10 min.
- Encoder failure rate > 5% for 15 min.
- Container restart loop.

Wire to PagerDuty/Slack via Grafana Alerting (provisioned under
`observability/grafana/provisioning/alerting/`).

## Traces

- OTel SDK initialized in `internal/observability` with the
  configured `OTEL_*` env vars.
- Sampler: `parentbased_traceidratio` with rate `OTEL_SAMPLER_ARG`
  (default `0.1` in prod, `1.0` in dev).
- Exporter: OTLP gRPC to `OTEL_EXPORTER_OTLP_ENDPOINT`
  (`http://otel-collector:4317` in Compose).
- Auto-instrumentation: `otelhttp` on the chi router; AWS SDK calls
  wrapped via `otelaws`; Redis via `redisotel` (when configured).
- Storage: Tempo (`observability/tempo.yaml`) with S3 backend
  (`bi8s-storage-*/tempo/`).

Trace IDs are echoed to logs via the slog handler, so you can pivot
from a Loki log line to the Tempo trace in one click in Grafana.

## Grafana

- Provisioned via `observability/grafana/provisioning/`:
  `datasources/`, `dashboards/`, `alerting/`, `plugins/`.
- Default URL in Compose: <http://localhost:3000>
  (`admin` / `admin` — change immediately).
- Dashboards include:
  - **API Overview**: RPS, error rate, p50/p95/p99 latency, top routes.
  - **Encoder**: jobs in flight, success/failure rate, ffmpeg duration.
  - **DynamoDB**: calls per table/op, throttles, latency.
  - **Runtime**: goroutines, GC pause, memory.

## Adding a new metric

```go
// in app/internal/observability or your service
counter, _ := otel.Meter("bi8s.encoder").Int64Counter(
    "bi8s_encoder_retries_total",
    metric.WithDescription("Encoder retries by reason."),
)
counter.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", "ffmpeg_oom")))
```

The Prometheus exporter on `:8889` picks it up automatically — no
scrape config changes needed.

## Adding a new trace span

```go
ctx, span := otel.Tracer("bi8s.repo.content").Start(ctx, "ContentRepo.Get")
defer span.End()
// ...
span.SetAttributes(attribute.String("content.id", id))
```

## See also

- [RUNBOOK.md](RUNBOOK.md) — how to use these signals during an incident.
- `observability/grafana/provisioning/dashboards/` — JSON dashboards.
- Saved tip: `/memories/repo/grafana-dashboard-metrics.md`.
