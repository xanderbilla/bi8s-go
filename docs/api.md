# API

Public REST surface served by `bi8s-go`. The machine-readable spec is
[`openapi.yaml`](openapi.yaml) (also served at `GET /v1/openapi.yaml` and
rendered by Swagger UI at `GET /v1/docs`).

## Base path

All endpoints are namespaced under **`/v1`**. Two route groups exist:

| Group    | Prefix    | Audience                                                     |
| -------- | --------- | ------------------------------------------------------------ |
| Consumer | `/v1/c/*` | Public reads (no auth required at the API layer today).      |
| Admin    | `/v1/a/*` | Writes + administrative reads (front by NGINX/auth in prod). |

The router is defined in [app/internal/http/router.go](../app/internal/http/router.go).

## Response envelope

Every endpoint — success **and** failure — returns the same JSON shape.
Source: [app/internal/response/response.go](../app/internal/response/response.go).

```json
{
  "success":   true,
  "status":    200,
  "message":   "ok",
  "data":      { ... },
  "error":     null,
  "path":      "/v1/c/content/abc123",
  "requestId": "01HZX...",
  "timestamp": "2025-05-04T04:30:00Z"
}
```

On error, `data` is `null` and `error` is populated:

```json
{
  "success": false,
  "status": 404,
  "message": "Content not found",
  "data": null,
  "error": {
    "type": "NOT_FOUND_ERROR",
    "code": "NOT_FOUND",
    "title": "Content not found",
    "detail": "no content with id abc123",
    "userMessage": "We couldn't find that content.",
    "context": null
  },
  "path": "/v1/c/content/abc123",
  "requestId": "01HZX...",
  "timestamp": "2025-05-04T04:30:00Z"
}
```

`error.type` is one of: `VALIDATION_ERROR`, `AUTH_ERROR`, `NOT_FOUND_ERROR`,
`CONFLICT_ERROR`, `RATE_LIMIT_ERROR`, `SERVER_ERROR`, `CLIENT_ERROR`.

`error.code` is a stable, machine-readable string (e.g. `NOT_FOUND`,
`RATE_LIMITED`, `INVALID_REQUEST`, `METHOD_NOT_ALLOWED`).

## Headers

| Header                            | Direction                              | Purpose                                                    |
| --------------------------------- | -------------------------------------- | ---------------------------------------------------------- |
| `X-Request-ID`                    | request (optional) / response (always) | Correlate logs/traces. Server generates one if absent.     |
| `Content-Type: application/json`  | both                                   | Required on requests with bodies; always set on responses. |
| `Access-Control-Allow-Origin`     | response                               | CORS — see `CORS_ALLOWED_ORIGINS`.                         |
| `Strict-Transport-Security`       | response                               | HSTS (1 year, preload).                                    |
| `X-Content-Type-Options: nosniff` | response                               | Disable MIME sniffing.                                     |
| `X-Frame-Options: DENY`           | response                               | Clickjacking protection.                                   |
| `Referrer-Policy: no-referrer`    | response                               | Suppress referrer leakage.                                 |
| `Retry-After`                     | response (429)                         | Seconds until rate-limit window resets.                    |

## Route table

### Health & docs (`/v1`)

| Method | Path               | Purpose                                                     |
| ------ | ------------------ | ----------------------------------------------------------- |
| `GET`  | `/v1/health`       | Aggregate health (`HealthChecks` from `bootstrap.go`).      |
| `GET`  | `/v1/livez`        | Liveness probe (always 200 if the process is up).           |
| `GET`  | `/v1/readyz`       | Readiness probe (200 only when dependencies are reachable). |
| `GET`  | `/v1/openapi.yaml` | Embedded OpenAPI 3.0 spec.                                  |
| `GET`  | `/v1/docs`         | Swagger UI (loads the spec above).                          |

### Consumer (`/v1/c`)

| Method | Path                              | Purpose                                   |
| ------ | --------------------------------- | ----------------------------------------- |
| `GET`  | `/content`                        | Recently added content.                   |
| `GET`  | `/content/{contentId}`            | Single content item by id.                |
| `GET`  | `/people/{peopleId}`              | Person detail.                            |
| `GET`  | `/people/{peopleId}/content`      | Content credited to a person.             |
| `GET`  | `/banner`                         | Curated banner content for the home page. |
| `GET`  | `/attributes/{id}`                | Content tagged with the given attribute.  |
| `GET`  | `/discover`                       | Discovery feed.                           |
| `GET`  | `/play/{contentType}/{contentId}` | Playback manifest URL + DRM metadata.     |

### Admin (`/v1/a`)

| Method   | Path                         | Purpose                                                 |
| -------- | ---------------------------- | ------------------------------------------------------- |
| `POST`   | `/content/{contentId}`       | Upload assets (multipart) for an existing content item. |
| `POST`   | `/encoder`                   | Submit an encoding job (write rate-limited).            |
| `GET`    | `/encoder/{jobId}`           | Encoding job status.                                    |
| `GET`    | `/content`                   | List all content.                                       |
| `GET`    | `/content/{contentId}`       | Admin content detail.                                   |
| `POST`   | `/content`                   | Create content (write rate-limited).                    |
| `DELETE` | `/content/{contentId}`       | Delete content.                                         |
| `GET`    | `/people`                    | List all people.                                        |
| `GET`    | `/people/{peopleId}`         | Admin person detail.                                    |
| `POST`   | `/people`                    | Create person (write rate-limited).                     |
| `DELETE` | `/people/{peopleId}`         | Delete person.                                          |
| `GET`    | `/people/{peopleId}/content` | Admin: content credited to a person.                    |
| `GET`    | `/attributes`                | List attributes.                                        |
| `GET`    | `/attributes/{attributeId}`  | Single attribute.                                       |
| `POST`   | `/attributes`                | Create attribute.                                       |
| `DELETE` | `/attributes/{attributeId}`  | Delete attribute.                                       |

## Validation

Path parameters are validated by `ValidateURLParams(...)` middleware (see
[app/internal/http/validation_middleware.go](../app/internal/http/validation_middleware.go)).
Bodies are validated with `go-playground/validator`, including custom
`customdate` (RFC 3339 date) and `daterange` (start ≤ end) tags defined in
`internal/validation`.

A failed validation always produces a `422 Unprocessable Entity` envelope
with `error.type = VALIDATION_ERROR` and `error.context` carrying the field
errors.

## Rate limits

Defaults (override via env vars; see [CONFIGURATION.md](CONFIGURATION.md)):

| Bucket                        | Burst | RPM | Env vars                                             |
| ----------------------------- | ----- | --- | ---------------------------------------------------- |
| Global (all routes)           | 100   | 100 | `RATELIMIT_GLOBAL_BURST`, `RATELIMIT_GLOBAL_PER_MIN` |
| `POST /v1/a/encoder`          | 5     | 5   | `RATELIMIT_ENCODER_WRITE_*`                          |
| `POST/DELETE /v1/a/content/*` | 20    | 20  | `RATELIMIT_MOVIE_WRITE_*`                            |
| `POST/DELETE /v1/a/people/*`  | 20    | 20  | `RATELIMIT_PERSON_WRITE_*`                           |

When tripped, the API returns `429` with `error.code = RATE_LIMITED` and a
`Retry-After` header.

## Pagination & limits

- Request body cap: `HTTP_MAX_JSON_BYTES` (default 1 MiB).
- DynamoDB scan cap: `DYNAMODB_MAX_SCAN_PAGES` (default 1000).
- Per-request timeout: `ROUTER_TIMEOUT_SECONDS` (default 60).

See [`openapi.yaml`](openapi.yaml) for full request/response schemas.
