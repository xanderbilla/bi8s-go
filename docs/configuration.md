# Configuration

`bi8s-go` reads its configuration exclusively from environment variables.
Defaults live in [`app/internal/app/config.go`](../app/internal/app/config.go);
validation lives in `Config.Validate()` and runs at startup — the binary
will refuse to start with an invalid config.

`.env.example` (in the repo root) is the canonical, copy-pasteable template.
Copy it to `.env` and fill in real values. **Never commit `.env`.**

## Required vs optional

A variable is **required** if `Config.Validate()` rejects an empty value.
Everything else is optional with the default shown.

### Application

| Variable         | Required | Default | Notes                                                                 |
| ---------------- | -------- | ------- | --------------------------------------------------------------------- |
| `APP_ENV`        | yes      | —       | `dev`, `staging`, or `prod`. Controls log format and CORS strictness. |
| `PORT`           | yes      | —       | Listen address (e.g. `:8080`).                                        |
| `LOG_LEVEL`      | no       | `info`  | `debug`, `info`, `warn`, `error`.                                     |
| `LOG_ADD_SOURCE` | no       | `false` | Add `source={file:line}` to log lines.                                |
| `BUILD_VERSION`  | no       | `dev`   | Surfaced in `/v1/health` and OTel resource attributes.                |

### HTTP / CORS

| Variable                     | Required | Default                  | Notes                                                                                                              |
| ---------------------------- | -------- | ------------------------ | ------------------------------------------------------------------------------------------------------------------ |
| `CORS_ALLOWED_ORIGINS`       | no       | see `DefaultCORSOrigins` | Comma-separated. **Cannot contain `*`** when credentials are enabled. In `prod`, every origin must use `https://`. |
| `CORS_ALLOW_PRIVATE_NETWORK` | no       | `false`                  | Enable [Private Network Access](https://wicg.github.io/private-network-access/) preflight.                         |
| `HTTP_MAX_JSON_BYTES`        | no       | `1048576`                | Per-request body cap (bytes).                                                                                      |
| `ROUTER_TIMEOUT_SECONDS`     | no       | `60`                     | Per-request deadline.                                                                                              |
| `TRUSTED_PROXIES`            | no       | (empty)                  | Comma-separated CIDRs. Required when behind NGINX so `X-Forwarded-For` is honoured.                                |

### AWS

| Variable                | Required | Default | Notes                                                                      |
| ----------------------- | -------- | ------- | -------------------------------------------------------------------------- |
| `AWS_REGION`            | yes      | —       | All AWS clients use this region.                                           |
| `AWS_ACCESS_KEY_ID`     | no¹      | —       | Falls back to the default credentials chain (instance profile, SSO, etc.). |
| `AWS_SECRET_ACCESS_KEY` | no¹      | —       | See above.                                                                 |

¹ Required only when not running under an EC2 instance profile / IRSA / SSO.

### DynamoDB

| Variable                            | Required | Default      | Notes                                        |
| ----------------------------------- | -------- | ------------ | -------------------------------------------- |
| `DYNAMODB_MOVIE_TABLE`              | yes      | —            | Movie / content table name.                  |
| `DYNAMODB_PERSON_TABLE`             | yes      | —            | Person table name.                           |
| `DYNAMODB_ATTRIBUTE_TABLE`          | yes      | —            | Attribute table name.                        |
| `DYNAMODB_ENCODER_TABLE`            | yes      | —            | Encoder job table name.                      |
| `DYNAMODB_ATTRIBUTE_NAME_INDEX`     | no       | `name-index` | GSI used for attribute lookups by name.      |
| `DYNAMODB_ENCODER_CONTENT_ID_INDEX` | yes      | —            | GSI used to find encoder jobs by content id. |
| `DYNAMODB_MAX_SCAN_PAGES`           | no       | `1000`       | Hard cap on paginated scans.                 |
| `CTX_DB_TIMEOUT_MS`                 | no       | `30000`      | Per-DynamoDB-call deadline.                  |

### S3

| Variable           | Required | Default | Notes                                                         |
| ------------------ | -------- | ------- | ------------------------------------------------------------- |
| `S3_BUCKET`        | yes      | —       | Single bucket used for uploads, HLS, and Loki/Tempo backends. |
| `STORAGE_BASE_URL` | no       | (empty) | Public base URL prefix when generating links.                 |

### Encoder

| Variable                      | Required | Default | Notes                                   |
| ----------------------------- | -------- | ------- | --------------------------------------- |
| `ENCODER_MAX_CONCURRENT`      | no       | `2`     | Max parallel ffmpeg jobs.               |
| `ENCODER_FFMPEG_PARALLELISM`  | no       | `0`     | ffmpeg `-threads` (`0` = auto).         |
| `ENCODER_JOB_TIMEOUT_SECONDS` | no       | `1800`  | Hard timeout per encoding job.          |
| `BI8S_TMP_DIR`                | no       | `/tmp`  | Scratch directory used during encoding. |

### Rate limiting

| Variable                                     | Required    | Default     | Notes                                                                               |
| -------------------------------------------- | ----------- | ----------- | ----------------------------------------------------------------------------------- |
| `RATE_LIMIT_BACKEND`                         | no          | `memory`    | `memory` (per-instance) or `redis` (multi-replica safe).                            |
| `REDIS_URL`                                  | conditional | —           | Required when `RATE_LIMIT_BACKEND=redis`. Format: `redis://[:pass@]host:port[/db]`. |
| `RATE_LIMIT_REDIS_FAIL_MODE`                 | no          | `fail-open` | `fail-open` allows traffic on Redis outages; `fail-closed` rejects.                 |
| `RATE_LIMIT_REDIS_TIMEOUT_MS`                | no          | `50`        | Per-call deadline against Redis.                                                    |
| `RATELIMIT_GLOBAL_BURST`                     | no          | `100`       | Global token bucket burst.                                                          |
| `RATELIMIT_GLOBAL_PER_MIN`                   | no          | `100`       | Global refill rate (per minute).                                                    |
| `RATELIMIT_ENCODER_WRITE_BURST` / `_PER_MIN` | no          | `5` / `5`   | Burst / RPM for `POST /v1/a/encoder`.                                               |
| `RATELIMIT_MOVIE_WRITE_BURST` / `_PER_MIN`   | no          | `20` / `20` | Burst / RPM for movie write routes.                                                 |
| `RATELIMIT_PERSON_WRITE_BURST` / `_PER_MIN`  | no          | `20` / `20` | Burst / RPM for person write routes.                                                |

### OpenTelemetry

| Variable                              | Required | Default               | Notes                                        |
| ------------------------------------- | -------- | --------------------- | -------------------------------------------- |
| `OTEL_ENABLED`                        | no       | `true`                | Master switch.                               |
| `OTEL_SERVICE_NAME`                   | no       | `bi8s-api`            | Resource attribute.                          |
| `OTEL_EXPORTER_OTLP_ENDPOINT`         | no       | `otel-collector:4317` | gRPC endpoint of the collector.              |
| `OTEL_EXPORTER_OTLP_INSECURE`         | no       | `true`                | Set `false` when the collector requires TLS. |
| `OTEL_TRACES_ENABLED`                 | no       | `true`                | Disable trace export only.                   |
| `OTEL_METRICS_ENABLED`                | no       | `true`                | Disable metric export only.                  |
| `OTEL_TRACES_SAMPLER_ARG`             | no       | `1.0`                 | Head-based sampler ratio (0–1).              |
| `OTEL_METRIC_EXPORT_INTERVAL_SECONDS` | no       | `15`                  | Periodic exporter interval.                  |
| `OTEL_SHUTDOWN_TIMEOUT_SECONDS`       | no       | `5`                   | Max time to flush on shutdown.               |

### Local-stack-only (compose)

These are read by `docker-compose.local.yml`, not by the binary.

| Variable                                                       | Default            | Purpose                                              |
| -------------------------------------------------------------- | ------------------ | ---------------------------------------------------- |
| `API_PORT` / `UI_PORT` / `REDIS_PORT` / `OTEL_GRPC_PORT` / ... | see `.env.example` | Host port mappings.                                  |
| `PROMETHEUS_RETENTION`                                         | `72h`              | Prometheus TSDB retention.                           |
| `GRAFANA_ADMIN_USER` / `GRAFANA_ADMIN_PASSWORD`                | `admin` / `admin`  | Grafana login (change in any non-local environment). |

## Validation rules (enforced at startup)

`Config.Validate()` will fail fast on:

- Unknown `APP_ENV` (must be `dev`, `staging`, or `prod`).
- Empty `PORT`, `S3_BUCKET`, `AWS_REGION`, any `DYNAMODB_*_TABLE`, or
  `DYNAMODB_ENCODER_CONTENT_ID_INDEX`.
- `RATE_LIMIT_BACKEND=redis` with empty `REDIS_URL`.
- `CORS_ALLOWED_ORIGINS=*` (must enumerate explicit origins).
- Any non-`https://` origin when `APP_ENV=prod`.

## Tips

- Use [`direnv`](https://direnv.net/) with `.envrc` to auto-load `.env`.
- Two profiles ship out of the box: `.env.example` (full reference) and
  `.envrc.example` (minimal direnv setup).
- The Compose file uses `env_file: .env` with `required: false`, so missing
  files won't break local-stack-only flows that don't need the API.
