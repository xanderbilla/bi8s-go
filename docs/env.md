# Environment Variables

## Required (all environments)

| Variable                            | Description                                                                       |
| ----------------------------------- | --------------------------------------------------------------------------------- |
| `S3_BUCKET`                         | S3 bucket for asset/HLS storage.                                                  |
| `AWS_REGION`                        | AWS region (default `us-east-1`).                                                 |
| `DYNAMODB_ENCODER_CONTENT_ID_INDEX` | Name of the encoder GSI keyed by `contentId`. **Server fails to start if unset.** |

## Required in production (`APP_ENV=prod`)

| Variable          | Description                                                                                                                                                                          |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `TRUSTED_PROXIES` | Comma-separated CIDRs of your load balancer / CDN. Without this, `X-Forwarded-For` is ignored and rate limiting falls back to peer IP. **Server fails to start in `prod` if unset.** |

## Optional

| Variable                     | Default              | Notes                                                                                                                     |
| ---------------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `APP_ENV`                    | `prod`               | One of `dev`, `staging`, `prod` (`development`/`production` accepted).                                                    |
| `PORT`                       | `:8080`              | Listen address.                                                                                                           |
| `LOG_LEVEL`                  | `info`               | `debug`, `info`, `warn`, `error`.                                                                                         |
| `LOG_ADD_SOURCE`             | `false`              | Include source file/line in logs.                                                                                         |
| `CORS_ALLOWED_ORIGINS`       | localhost set        | Comma-separated. **`*` is rejected** (credentials are allowed).                                                           |
| `CORS_ALLOW_PRIVATE_NETWORK` | `true`               |                                                                                                                           |
| `DYNAMODB_MOVIE_TABLE`       | `bi8s-dev`           |                                                                                                                           |
| `DYNAMODB_PERSON_TABLE`      | `bi8s-person-dev`    |                                                                                                                           |
| `DYNAMODB_ATTRIBUTE_TABLE`   | `bi8s-attribute-dev` |                                                                                                                           |
| `DYNAMODB_ENCODER_TABLE`     | `bi8s-video-dev`     |                                                                                                                           |
| `STARTUP_HEALTH_CHECK`       | `true`               | When `true`, all registered health checks must pass before the process accepts traffic. Set to `false` in local dev only. |

## HTTP server timeouts & limits

| Variable                              | Default       | Notes                                                           |
| ------------------------------------- | ------------- | --------------------------------------------------------------- |
| `INIT_TIMEOUT_SECONDS`                | `30`          | Bound on AWS client / repo initialization.                      |
| `STARTUP_HEALTHCHECK_TIMEOUT_SECONDS` | `10`          | Bound on registered health checks during startup probe.         |
| `HTTP_READ_HEADER_TIMEOUT_SECONDS`    | `5`           | `http.Server.ReadHeaderTimeout`.                                |
| `HTTP_READ_TIMEOUT_SECONDS`           | `30`          | `http.Server.ReadTimeout`.                                      |
| `HTTP_WRITE_TIMEOUT_SECONDS`          | `65`          | `http.Server.WriteTimeout` (must exceed router timeout).        |
| `HTTP_IDLE_TIMEOUT_SECONDS`           | `120`         | `http.Server.IdleTimeout`.                                      |
| `HTTP_MAX_HEADER_BYTES`               | `1048576`     | `http.Server.MaxHeaderBytes` (1 MiB).                           |
| `SHUTDOWN_TIMEOUT_SECONDS`            | `30`          | `http.Server.Shutdown` deadline.                                |
| `ENCODER_DRAIN_TIMEOUT_SECONDS`       | `120`         | Bound on draining in-flight encoder jobs after shutdown signal. |
| `ENCODER_JOB_TIMEOUT_SECONDS`         | `1800`        | Per-job context deadline used by the encoder service worker.    |
| `ROUTER_TIMEOUT_SECONDS`              | `60`          | Per-request handler timeout (chi `middleware.Timeout`).         |
| `HTTP_MAX_JSON_BYTES`                 | `1048576`     | Max JSON request body (1 MiB).                                  |
| `HTTP_MAX_REQUEST_BODY_BYTES`         | `1048576`     | Generic JSON-style body cap.                                    |
| `HTTP_MAX_MULTIPART_BODY_BYTES`       | `12582912`    | Multipart upload body cap.                                      |
| `HTTP_MAX_MULTIPART_FILE_BYTES`       | `10485760`    | Per-file cap inside multipart uploads.                          |
| `HTTP_MAX_VIDEO_BODY_BYTES`           | `10737418240` | Total body cap for video uploads (10 GiB).                      |
| `HTTP_MAX_VIDEO_FILE_BYTES`           | `10737418240` | Per-file cap for video uploads.                                 |

## Rate limiting

When a request is throttled, the limiter returns HTTP 429 with the standard
envelope and these response headers:

| Header              | Notes                                                                |
| ------------------- | -------------------------------------------------------------------- |
| `Retry-After`       | Seconds the client should wait before retrying.                      |
| `X-RateLimit-Limit` | Configured per-minute rate (tokens / minute) for the matched bucket. |

| Variable                          | Default | Notes                                 |
| --------------------------------- | ------- | ------------------------------------- |
| `RATELIMIT_GLOBAL_BURST`          | `100`   | Global burst capacity (tokens).       |
| `RATELIMIT_GLOBAL_PER_MIN`        | `100`   | Global refill rate (tokens / minute). |
| `RATELIMIT_ENCODER_WRITE_BURST`   | `5`     | Encoder write burst.                  |
| `RATELIMIT_ENCODER_WRITE_PER_MIN` | `5`     | Encoder write refill / minute.        |
| `RATELIMIT_MOVIE_WRITE_BURST`     | `20`    | Movie write burst.                    |
| `RATELIMIT_MOVIE_WRITE_PER_MIN`   | `20`    | Movie write refill / minute.          |
| `RATELIMIT_PERSON_WRITE_BURST`    | `20`    | Person write burst.                   |
| `RATELIMIT_PERSON_WRITE_PER_MIN`  | `20`    | Person write refill / minute.         |

## Context timeouts

| Variable                 | Default | Notes                              |
| ------------------------ | ------- | ---------------------------------- |
| `CTX_DB_TIMEOUT_MS`      | `5000`  | `ctxutil.WithDBTimeout` deadline.  |
| `CTX_S3_TIMEOUT_MS`      | `30000` | `ctxutil.WithS3Timeout` deadline.  |
| `CTX_API_TIMEOUT_MS`     | `10000` | `ctxutil.WithAPITimeout` deadline. |
| `CTX_LONG_OP_TIMEOUT_MS` | `60000` | Long-running operation cap.        |

## DynamoDB

| Variable                  | Default | Notes                                                          |
| ------------------------- | ------- | -------------------------------------------------------------- |
| `DYNAMODB_MAX_SCAN_PAGES` | `10`    | Max pages walked by paginated scans (defence against runaway). |

## CORS

| Variable               | Default                                          | Notes                                                                                        |
| ---------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| `DEFAULT_CORS_ORIGINS` | `http://localhost:3000,…,https://127.0.0.1:8443` | Used only when `CORS_ALLOWED_ORIGINS` is unset. Same rules apply (`*` rejected, prod=https). |

## Sensitive (read but never logged)

| Variable                | Description                 |
| ----------------------- | --------------------------- |
| `AWS_ACCESS_KEY_ID`     | Optional; prefer IAM roles. |
| `AWS_SECRET_ACCESS_KEY` | Optional; prefer IAM roles. |

## AWS retry behaviour

The AWS SDK is configured with **adaptive retry mode** and **5 max attempts** in `internal/aws/config.go`. Throttled DynamoDB calls (`ProvisionedThroughputExceededException`, `RequestLimitExceeded`) can be detected with `errs.IsThrottled`.
