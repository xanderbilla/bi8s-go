# Observability

The bi8s-go service emits **traces, metrics, and logs** via the OpenTelemetry
SDK to a local **OpenTelemetry Collector**, which fans the data out to
**Tempo** (traces), **Prometheus** (metrics), and **Loki** (logs). Grafana is
the single pane of glass.

> **Strict rule:** the Go application talks to the OTel Collector **only**.
> It never connects directly to Prometheus, Tempo, or Loki.

```
┌─────────────┐  OTLP/gRPC :4317  ┌──────────────────┐  ┌────────────┐
│  bi8s-api   │ ────────────────► │  otel-collector  │─►│   tempo    │
└─────────────┘                   │                  │  └────────────┘
                                  │                  │  ┌────────────┐
                                  │                  │─►│ prometheus │  (scrape :8889)
                                  │                  │  └────────────┘
                                  │                  │  ┌────────────┐
                                  └──────────────────┘─►│    loki    │
                                                        └────────────┘
                                  ┌──────────────────┐
   docker container logs    ────► │     promtail     │── push ──► loki
                                  └──────────────────┘
```

## Components

| Service          | Port (host)        | Purpose                                        |
| ---------------- | ------------------ | ---------------------------------------------- |
| `api`            | 8080               | Go service emitting OTLP                       |
| `otel-collector` | 4317 / 4318 / 8889 | OTLP gRPC / HTTP / Prom exposition             |
| `prometheus`     | 9090               | Metrics TSDB + remote-write receiver           |
| `tempo`          | 3200               | Distributed tracing backend                    |
| `loki`           | 3100               | Logs backend                                   |
| `promtail`       | —                  | Tails docker container logs into Loki          |
| `grafana`        | 3000               | UI; pre-provisioned datasources & correlations |

## SDK configuration

All knobs are environment variables (see [`.env.example`](../.env.example)).

| Variable                              | Default               | Notes                                        |
| ------------------------------------- | --------------------- | -------------------------------------------- |
| `OTEL_SERVICE_NAME`                   | `bi8s-api`            | Set on `service.name` resource attr          |
| `OTEL_ENABLED`                        | `true`                | Master switch — disables SDK init entirely   |
| `OTEL_EXPORTER_OTLP_ENDPOINT`         | `otel-collector:4317` | gRPC endpoint of the collector               |
| `OTEL_EXPORTER_OTLP_INSECURE`         | `true`                | Use plaintext gRPC (local only)              |
| `OTEL_TRACES_ENABLED`                 | `true`                | Toggle trace exporter                        |
| `OTEL_METRICS_ENABLED`                | `true`                | Toggle metric exporter                       |
| `OTEL_TRACES_SAMPLER_ARG`             | `1.0`                 | `ParentBased(TraceIDRatioBased)` ratio (0–1) |
| `OTEL_METRIC_EXPORT_INTERVAL_SECONDS` | `15`                  | Periodic reader interval                     |
| `OTEL_SHUTDOWN_TIMEOUT_SECONDS`       | `5`                   | Graceful flush timeout on exit               |
| `BUILD_VERSION`                       | `dev`                 | Set on `service.version` resource attr       |

## What is instrumented

- **HTTP server** — every request is wrapped by `otelhttp.NewHandler`,
  producing a span named `<METHOD> <route-template>` and recording
  `http.server.requests`, `http.server.duration`, `http.server.active_requests`,
  and `http.server.response.body.size` metrics with `http.route`,
  `http.request.method`, and `http.response.status_code` attributes.
- **AWS SDK v2** — `otelaws.AppendMiddlewares` is applied to every client
  built from `internal/aws.LoadConfig`, so DynamoDB and S3 calls become child
  spans of the originating HTTP request.
- **Logs** — the global `slog` JSON handler is wrapped by
  `observability.SlogHandler`, which automatically injects `trace_id` and
  `span_id` attributes whenever a record is emitted with a context that has an
  active span. This is what makes the Loki ↔ Tempo correlation work.

## Grafana correlations

Datasources are provisioned in
`observability/grafana/provisioning/datasources/datasources.yaml`:

- **Tempo → Logs**: clicking a span jumps to the matching Loki window via
  `tracesToLogsV2`.
- **Tempo → Metrics**: span/service metrics jump back to Prometheus via
  `tracesToMetrics`.
- **Loki → Traces**: derived field `trace_id` turns every log line containing
  a trace ID into a clickable link to the Tempo trace.
- **Prometheus exemplars**: enabled so RED-style histograms can pop out
  exemplar trace IDs.

## Running locally

```sh
cp .env.example .env          # fill AWS creds and table names
docker compose -f docker-compose.local.yml up -d
```

| URL                             | Service                       |
| ------------------------------- | ----------------------------- |
| <http://localhost:8080/healthz> | API health                    |
| <http://localhost:4000>         | Grafana (default admin/admin) |
| <http://localhost:9090>         | Prometheus                    |
| <http://localhost:3200>         | Tempo                         |
| <http://localhost:3100/ready>   | Loki                          |

To run **without** the UI service, omit the `ui` profile (it is opt-in):

```sh
docker compose -f docker-compose.local.yml up -d           # api + observability
docker compose -f docker-compose.local.yml --profile ui up # add UI
```

## Dashboard persistence

Grafana dashboards are provisioned from repository files on every startup:

- `observability/grafana/provisioning/dashboards/files/logging.json`
- `observability/grafana/provisioning/dashboards/files/metrics.json`

The compose file mounts both files into Grafana's provisioning path
`/etc/grafana/provisioning/dashboards/files`, so dashboards are recreated even
after a full cleanup such as:

```sh
docker compose -f docker-compose.local.yml down -v
docker compose -f docker-compose.local.yml up -d
```

If you edit dashboard JSON in Git, restart Grafana (or `up -d` again) to load
the updated definitions.

## Disabling telemetry

Set `OTEL_ENABLED=false` on the Go process. The SDK initializes a noop
provider; no network calls are attempted, and the binary keeps running.

## Troubleshooting

| Symptom                      | Likely cause                                   | Fix                                                                 |
| ---------------------------- | ---------------------------------------------- | ------------------------------------------------------------------- |
| No traces in Tempo           | App can't reach collector                      | Check `OTEL_EXPORTER_OTLP_ENDPOINT` and that `otel-collector` is up |
| No `trace_id` in logs        | Request handler didn't propagate `r.Context()` | Always pass `r.Context()` into downstream calls                     |
| Prometheus shows zero series | Collector → Prom scrape misconfigured          | `curl http://localhost:8889/metrics` should return data             |
| Loki has no logs             | Promtail can't reach docker socket             | Ensure `/var/run/docker.sock` is mounted; check promtail logs       |

## Grafana dashboard panels

The sections below list every panel that can be built in Grafana, grouped by
datasource. Each entry shows the panel title, the query to paste in, and what
it reveals. All panels target the `bi8s` service; adjust label selectors if
the service name changes.

---

### Prometheus panels

Metrics namespace is `bi8s`. Scraped from `otel-collector:8889`, job label
`otel-collector`.

Key label names confirmed live:

| Label                       | Sample values                |
| --------------------------- | ---------------------------- |
| `service_name`              | `bi8s-api`                   |
| `http_request_method`       | `GET`, `POST`                |
| `http_response_status_code` | `200`, `404`, `500`          |
| `http_route`                | `/v1/c/content`, `unmatched` |
| `deployment_environment`    | `dev`                        |

---

#### Panel 1 — Total Request Count

**Visualization:** Stat

```promql
sum(bi8s_http_server_requests_total{service_name="bi8s-api"})
```

Lifetime request counter across all routes and methods. Use as a heartbeat
indicator — if the number stops rising, the app has stalled or lost traffic.

---

#### Panel 2 — Request Rate (RPS)

**Visualization:** Time series

```promql
sum(rate(bi8s_http_server_requests_total{service_name="bi8s-api"}[5m]))
```

Requests per second over a 5-minute rolling window. This is the **R** in RED
metrics. Pair with the error rate panel to catch traffic surges that are also
degrading.

---

#### Panel 3 — Request Rate by Route

**Visualization:** Time series

```promql
sum by (http_route) (
  rate(bi8s_http_server_requests_total{service_name="bi8s-api"}[5m])
)
```

Breaks RPS down per route template. Shows which endpoints are hot and helps
detect unexpected traffic distribution (e.g. a hammered `/v1/c/content` while
`/v1/m/movies` is idle).

---

#### Panel 4 — Error Rate (5xx)

**Visualization:** Time series

```promql
sum(rate(bi8s_http_server_requests_total{
  service_name="bi8s-api",
  http_response_status_code=~"5.."
}[5m]))
```

Rate of server-error responses per second — the **E** in RED. Any sustained
value above 0 in production warrants immediate attention.

---

#### Panel 5 — Error Ratio (%)

**Visualization:** Gauge or Stat

```promql
sum(rate(bi8s_http_server_requests_total{
  service_name="bi8s-api",
  http_response_status_code=~"5.."
}[5m]))
/
sum(rate(bi8s_http_server_requests_total{service_name="bi8s-api"}[5m]))
* 100
```

Percentage of requests that are errors. Easier to alert on than an absolute
rate because it self-normalises at different traffic volumes.

---

#### Panel 6 — HTTP Status Distribution

**Visualization:** Bar chart or Pie chart

```promql
sum by (http_response_status_code) (
  rate(bi8s_http_server_requests_total{service_name="bi8s-api"}[5m])
)
```

Breaks traffic by status code. Shows the ratio of 2xx success, 4xx client
errors, and 5xx server errors at a glance.

---

#### Panel 7 — P50 Request Duration

**Visualization:** Time series

```promql
histogram_quantile(0.50,
  sum by (le) (
    rate(bi8s_http_server_duration_milliseconds_bucket{service_name="bi8s-api"}[5m])
  )
)
```

Median latency in milliseconds — the typical user experience. Compare against
P95 and P99 to understand how spread out the latency distribution is.

---

#### Panel 8 — P95 Request Duration

**Visualization:** Time series

```promql
histogram_quantile(0.95,
  sum by (le) (
    rate(bi8s_http_server_duration_milliseconds_bucket{service_name="bi8s-api"}[5m])
  )
)
```

The **D** in RED metrics — the 95th percentile of request latency. This is the
number that represents the slowest 1-in-20 requests and is the standard SLO
target.

---

#### Panel 9 — P95 Duration by Route

**Visualization:** Time series

```promql
histogram_quantile(0.95,
  sum by (le, http_route) (
    rate(bi8s_http_server_duration_milliseconds_bucket{service_name="bi8s-api"}[5m])
  )
)
```

Per-route P95. Reveals that one slow endpoint (e.g. `GET /v1/c/content` with a
DynamoDB Scan) is inflating the aggregate P95 while others are fast.

---

#### Panel 10 — Active (In-Flight) Requests

**Visualization:** Time series or Stat

```promql
sum(bi8s_http_server_active_requests{service_name="bi8s-api"})
```

Real-time count of requests currently being processed. Sustained high values
under normal RPS indicate a slow downstream (DynamoDB, S3) causing request
piling. Drops to 0 between requests under light load.

---

#### Panel 11 — Active Requests by Route

**Visualization:** Time series

```promql
sum by (http_route) (
  bi8s_http_server_active_requests{service_name="bi8s-api"}
)
```

Shows which specific routes are backed up. If one route consistently holds more
in-flight requests than others, that route has a bottleneck worth investigating
in Tempo.

---

#### Panel 12 — Total Response Bytes Served

**Visualization:** Stat

```promql
sum(bi8s_http_server_response_body_size_bytes_total{service_name="bi8s-api"})
```

Cumulative bytes sent to clients. Use as a bandwidth / egress cost indicator.

---

#### Panel 13 — Response Throughput (bytes/sec)

**Visualization:** Time series

```promql
sum(rate(bi8s_http_server_response_body_size_bytes_total{service_name="bi8s-api"}[5m]))
```

Outbound data rate. Combine with request rate to derive average response
payload size. Useful for detecting unexpectedly large responses (e.g. a
missing pagination limit).

---

#### Panel 14 — Response Throughput by Route

**Visualization:** Time series

```promql
sum by (http_route) (
  rate(bi8s_http_server_response_body_size_bytes_total{service_name="bi8s-api"}[5m])
)
```

Per-route bandwidth. Helps identify which endpoints dominate egress cost.

---

### Loki panels

Loki label selectors:

```
{container="bi8s_api_local"}         — all lines from the api container
{container="bi8s_api_local"} | json  — parse json fields for filtering
```

Log line JSON schema (fields present in every `http_request` log):

| Field         | Example                                 |
| ------------- | --------------------------------------- |
| `time`        | `2026-05-02T09:17:45Z`                  |
| `level`       | `INFO`, `WARN`, `ERROR`                 |
| `msg`         | `http_request`, `starting server`       |
| `request_id`  | `640a5a42-...`                          |
| `method`      | `GET`                                   |
| `path`        | `/v1/c/content`                         |
| `status`      | `200`, `404`, `500`                     |
| `duration_ms` | `12`                                    |
| `bytes`       | `881`                                   |
| `remote_addr` | `172.18.0.1`                            |
| `user_agent`  | `curl/8.7.1` _(optional)_               |
| `query`       | `limit=20` _(optional, when present)_   |
| `trace_id`    | `abc123...` _(injected by SlogHandler)_ |
| `span_id`     | `def456...` _(injected by SlogHandler)_ |

`trace_id` and `span_id` are injected automatically by `observability.SlogHandler`
whenever a log record is emitted inside an active OTel span context. This is the
field that powers the Loki → Tempo correlation link.

---

#### Panel 1 — All API Logs

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json
```

Live structured log stream from the API container. JSON-parsed so you can
filter interactively by level, path, status, or request_id in the Grafana
Logs panel.

---

#### Panel 2 — Error Logs

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | level="ERROR"
```

Only `ERROR`-level log lines. These correspond to 5xx HTTP responses, failed
encoder jobs, or unhandled panics caught by the Recoverer middleware.

---

#### Panel 3 — Warning Logs

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | level="WARN"
```

`WARN`-level logs: 4xx responses, misconfigured env vars, file cleanup
failures, and rate-limit events.

---

#### Panel 4 — Error Log Rate

**Visualization:** Time series

```logql
sum(rate({container="bi8s_api_local"} | json | level="ERROR" [5m]))
```

Rate of `ERROR`-level log lines per second. Spikes indicate 5xx bursts or
backend failures. Complements the Prometheus error rate panel with the actual
log content to diagnose the cause.

---

#### Panel 5 — Warning Log Rate

**Visualization:** Time series

```logql
sum(rate({container="bi8s_api_local"} | json | level="WARN" [5m]))
```

Rate of `WARN`-level log lines. A rising trend means 4xx spikes or
infrastructure noise (e.g. promtail-level docker restarts).

---

#### Panel 6 — Log Volume by Level (stacked)

**Visualization:** Time series (stacked)

```logql
sum by (level) (
  rate({container="bi8s_api_local"} | json [5m])
)
```

Stacked log rate broken down by `INFO` / `WARN` / `ERROR`. Shows the health
and noise ratio of the API at a glance — a healthy service has a tall INFO
band and very thin WARN/ERROR bands.

---

#### Panel 7 — Slow Requests (> 500ms)

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | duration_ms > 500
```

Log lines where the request took longer than 500ms. Directly shows which paths
are hitting DynamoDB slowly. Each line includes `request_id` and `trace_id`
for correlation.

---

#### Panel 8 — HTTP 5xx Responses

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | status >= 500
```

All requests that returned a 500-level status code. Includes path, method, and
`request_id` so you can trace the exact call that failed.

---

#### Panel 9 — HTTP 4xx Responses

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | status >= 400 | status < 500
```

Bad requests, not-found routes, unauthorized calls. Useful for spotting client
bugs or broken frontend path assumptions.

---

#### Panel 10 — Request Rate by Path (from logs)

**Visualization:** Time series

```logql
sum by (path) (
  rate({container="bi8s_api_local"} | json | msg="http_request" [5m])
)
```

Per-path request volume derived from logs. Complements the Prometheus RPS
panels — useful when you want to search for a specific path substring
rather than matching an exact route template.

---

#### Panel 11 — P95 Latency from Logs

**Visualization:** Time series

```logql
quantile_over_time(0.95,
  {container="bi8s_api_local"}
  | json
  | msg="http_request"
  | unwrap duration_ms [5m]
) by (path)
```

95th percentile `duration_ms` extracted directly from log fields, grouped by
path. Useful as a cross-check against the Prometheus histogram panel.

---

#### Panel 12 — Errors with Trace Links

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | level="ERROR" | trace_id != ""
```

`ERROR` logs that contain a `trace_id`. In Grafana, the derived field config on
the Loki datasource turns each `trace_id` value into a clickable "Tempo" button.
Click it to open the full distributed trace waterfall for that exact request.

---

#### Panel 13 — Request ID Search

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | request_id="<paste-id-here>"
```

Filter all logs for a single request. Turn `<paste-id-here>` into a Grafana
dashboard variable (`$request_id`) to make this an interactive search box.
Useful for debugging a specific user-reported failure end-to-end.

---

#### Panel 14 — API Lifecycle Events

**Visualization:** Logs

```logql
{container="bi8s_api_local"} | json | msg != "http_request"
```

Startup, shutdown, health check pass/fail, and other non-HTTP log messages.
Useful for detecting container restarts or configuration problems at boot time.

---

### Tempo panels

Tempo query language is **TraceQL**. Datasource UID `tempo`.

Span types recorded:

| Span name           | Kind     | Source                |
| ------------------- | -------- | --------------------- |
| `GET /v1/c/content` | `SERVER` | `otelhttp.NewHandler` |
| `GET /v1/m/movies`  | `SERVER` | `otelhttp.NewHandler` |
| `DynamoDB.Scan`     | `CLIENT` | `otelaws` middleware  |
| `DynamoDB/GetItem`  | `CLIENT` | `otelaws` middleware  |

**SERVER span attributes:** `http.request.method`, `url.path`, `url.scheme`,
`server.address`, `network.peer.address`, `client.address`,
`user_agent.original`, `network.protocol.version`, `http.response.status_code`

**CLIENT span attributes (DynamoDB):** `rpc.system`, `rpc.method`,
`aws.region`, `aws.request_id`, `db.system`, `aws.dynamodb.table_names`,
`aws.dynamodb.select`

---

#### Panel 1 — Service Graph

**Visualization:** Node Graph  
**Query type:** Service Graph (select datasource `tempo`, type → Service Graph)

Auto-generated map of services and their connections. Shows request rate, error
rate, and average duration between nodes. Updates live from span
parent-child relationships — no extra configuration required.

---

#### Panel 2 — Trace Explorer

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" }
```

Browse all recent traces from `bi8s-api`. Select any row to open the full
waterfall span view. The waterfall shows the `DynamoDB.Scan` child span under
the `GET /v1/c/content` parent, with the AWS round-trip duration visible.

---

#### Panel 3 — Slow Traces (> 500ms)

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" && duration > 500ms }
```

Root spans where total request duration exceeded 500ms. Live data shows
`GET /v1/c/content` traces hitting 346ms – 1175ms dominated by the
DynamoDB Scan child span.

---

#### Panel 4 — Error Traces

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" && status = error }
```

Traces where any span has status = ERROR. Use this to find 5xx responses that
have a full span context attached, then click through to the waterfall to see
which child span set the error status.

---

#### Panel 5 — DynamoDB Span Explorer

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" && span.db.system = "aws.dynamodb" }
```

All `CLIENT`-kind spans from DynamoDB — Scan and GetItem operations. Shows the
actual `aws.request_id`, table names, and per-call duration. Useful when you
need the AWS request ID for a support ticket.

---

#### Panel 6 — Slow DynamoDB Calls (> 200ms)

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" && span.db.system = "aws.dynamodb" && duration > 200ms }
```

DynamoDB calls that individually took more than 200ms. Your Scan spans are the
main offenders — this is where to look first for latency root cause. The Scan
duration accounts for ~95% of end-to-end `GET /v1/c/content` latency.

---

#### Panel 7 — Traces by HTTP Route

**Visualization:** Table / Traces list

```traceql
{ resource.service.name = "bi8s-api" && span.url.path = "/v1/c/content" }
```

All traces for a specific route. Change the `url.path` value to drill into any
endpoint individually.

---

#### Panel 8 — Request Rate from Traces

**Visualization:** Time series

```traceql
{ resource.service.name = "bi8s-api" && kind = server } | rate()
```

Rate of incoming server spans per second, derived from trace data. Cross-check
against the Prometheus RPS panel — if they diverge, there is a gap in the
collector pipeline or sampling is dropping spans.

---

#### Panel 9 — Error Rate from Traces

**Visualization:** Time series

```traceql
{ resource.service.name = "bi8s-api" && status = error } | rate()
```

Rate of ERROR-status spans over time. These are spans where `otelhttp`
detected a 5xx or your code explicitly set span status to error.

---

#### Panel 10 — P95 Latency from Traces

**Visualization:** Time series

```traceql
{ resource.service.name = "bi8s-api" && kind = server } | quantile_over_time(duration, 0.95)
```

95th percentile span duration calculated directly from trace data. More
accurate than Prometheus histograms because it operates on exact durations
rather than pre-bucketed estimates.

---

#### Panel 11 — P95 Latency by Route

**Visualization:** Time series

```traceql
{ resource.service.name = "bi8s-api" && kind = server } | quantile_over_time(duration, 0.95) by (span.url.path)
```

Per-route P95 latency. Shows which endpoints are slow independently of each
other. Directly reveals that `/v1/c/content` is dominated by its
`DynamoDB.Scan` child span.

---

## Cross-datasource comparison

| Question                                  | Prometheus        | Loki                       | Tempo                                               |
| ----------------------------------------- | ----------------- | -------------------------- | --------------------------------------------------- |
| Is error rate rising?                     | ✅ histogram rate | ✅ log rate                | ✅ `status = error \| rate()`                       |
| Which specific request was slow?          | ❌                | ✅ `duration_ms > 500`     | ✅ `duration > 500ms`                               |
| Why was that request slow?                | ❌                | ❌                         | ✅ waterfall — child span shows DynamoDB took 395ms |
| What did the app log during that request? | ❌                | ✅ filter by `trace_id`    | ✅ click → Loki link                                |
| How slow is DynamoDB specifically?        | ❌                | ❌                         | ✅ `span.db.system = "aws.dynamodb"`                |
| What was the AWS request ID?              | ❌                | ❌                         | ✅ `aws.request_id` attribute in span               |
| What is the app saying on startup?        | ❌                | ✅ `msg != "http_request"` | ❌                                                  |

---

## Production guidance

- Lower `OTEL_TRACES_SAMPLER_ARG` (e.g. `0.05`) to cap cardinality.
- Replace plaintext gRPC with TLS: set `OTEL_EXPORTER_OTLP_INSECURE=false`
  and point `OTEL_EXPORTER_OTLP_ENDPOINT` at a TLS-terminated collector.
- Run the collector as a sidecar (or DaemonSet) rather than a single shared
  instance.
- Configure retention policies on Tempo / Loki / Prometheus to match your
  storage budget.
