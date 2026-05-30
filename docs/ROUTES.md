# Routes

Canonical index of every HTTP route exposed by the binary. Source of truth is
[`app/internal/http/routes/v1.json`](../app/internal/http/routes/v1.json) and
the per-handler `Register` functions. Update this file whenever a route is
added, removed, or renamed.

- Machine-readable contract: [`openapi.yaml`](openapi.yaml)
- Response envelope & error shape: [`API.md`](API.md)

## Auth tiers

| Tier      | Description                                                                                                                                |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **None**  | Unauthenticated — open to all                                                                                                              |
| **Admin** | Internal network only; auth-stub middleware (`app/internal/http/middleware/authstub`). Swap in real auth without changing the route table. |

## Rate-limit classes

| Class           | Applied to           |
| --------------- | -------------------- |
| `encoder_write` | Encoder job creation |
| `movie_write`   | Content creation     |
| `person_write`  | People creation      |

---

## System / Operational

| Method | Path               | Handler              | Middlewares | Description                                                                                                                             |
| ------ | ------------------ | -------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/v1/health`       | `health.HealthCheck` | —           | Full health check. Concurrent checks against DynamoDB, OpenSearch, Redis, S3/B2, ffmpeg, ffprobe (2 s timeout). Returns `200` or `503`. |
| `GET`  | `/v1/livez`        | `health.Liveness`    | —           | Liveness probe. Always `200` if the process is alive.                                                                                   |
| `GET`  | `/v1/readyz`       | `health.Readiness`   | —           | Readiness probe. Returns `503 NOT_READY` until bootstrap completes, then behaves like `/health`.                                        |
| `GET`  | `/v1/openapi.yaml` | `docs.OpenAPISpec`   | —           | Serves the embedded OpenAPI 3.x YAML specification.                                                                                     |
| `GET`  | `/v1/docs`         | `docs.SwaggerUI`     | —           | Serves the embedded Swagger UI HTML page.                                                                                               |

---

## Consumer routes — `/v1/c/*`

No authentication required.

### Search

| Method | Path                                | Handler               | Middlewares                     | Description                                                                                                                                                                                                |
| ------ | ----------------------------------- | --------------------- | ------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/v1/c/search`                      | `search.Search`       | `timeout`                       | Full-text search across content and/or people. Required: `query`. Optional: `in` (`content`\|`people`), independent pagination (`contentPage`, `contentPageSize`, `peoplePage`, `peoplePageSize`), `sort`. |
| `GET`  | `/v1/c/content/{contentId}/related` | `search.MoreLikeThis` | `timeout`, `validate.contentId` | OpenSearch More-Like-This query returning content similar to the given item.                                                                                                                               |

### Content

| Method | Path                                   | Handler                  | Middlewares                               | Description                                                                                                                                                          |
| ------ | -------------------------------------- | ------------------------ | ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/v1/c/content/{contentId}`            | `content.GetConsumer`    | `timeout`, `validate.contentId`           | Public full detail for a single content item.                                                                                                                        |
| `GET`  | `/v1/c/banner`                         | `content.GetBanner`      | `timeout`                                 | Random featured/banner item for the hero carousel. Optional `?type=movie\|tv`.                                                                                       |
| `GET`  | `/v1/c/discover`                       | `content.GetDiscover`    | `timeout`, `httpcache.discover`           | Paginated discovery feed (`latest`/`popular`). In-memory cached for 2 min. Supports DynamoDB key-cursor pagination; switches to full-scan offset when `sort` is set. |
| `GET`  | `/v1/c/attributes/{id}`                | `content.GetByAttribute` | `timeout`, `validate.consumerAttributeId` | Paginated content list for a specific attribute (genre, tag, mood, etc.).                                                                                            |
| `GET`  | `/v1/c/play/{contentType}/{contentId}` | `content.GetPlayback`    | `timeout`, `validate.contentTypeAndId`    | HLS playback manifest: master playlist, quality tracks, audio, subtitles, thumbnails, preview, sprite.                                                               |

### People

| Method | Path                              | Handler                       | Middlewares                    | Description                                                                |
| ------ | --------------------------------- | ----------------------------- | ------------------------------ | -------------------------------------------------------------------------- |
| `GET`  | `/v1/c/people/{peopleId}`         | `person.GetConsumer`          | `timeout`, `validate.personId` | Public person profile.                                                     |
| `GET`  | `/v1/c/people/{peopleId}/content` | `content.GetByPersonConsumer` | `timeout`, `validate.personId` | Minimal paginated content list for a person (`{id, title, backdropPath}`). |

### Attributes

| Method | Path               | Handler                  | Middlewares | Description                                                                                     |
| ------ | ------------------ | ------------------------ | ----------- | ----------------------------------------------------------------------------------------------- |
| `GET`  | `/v1/c/attributes` | `attribute.ListConsumer` | `timeout`   | All active attributes, optionally filtered by `?type=` and sorted. In-memory cached for 10 min. |

---

## Admin routes — `/v1/a/*`

Require internal/admin network access (auth-stub middleware).

### Content

| Method   | Path                        | Handler                | Middlewares                   | Description                                                                                                                                                                                                                 |
| -------- | --------------------------- | ---------------------- | ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/v1/a/content/`            | `content.ListAdmin`    | `timeout`                     | Paginated list of all content including private items; full `Movie` objects with audit fields.                                                                                                                              |
| `GET`    | `/v1/a/content/{contentId}` | `content.GetAdmin`     | `timeout`, `validate.movieId` | Full admin detail for a content item.                                                                                                                                                                                       |
| `POST`   | `/v1/a/content/`            | `content.Create`       | `timeout`, `ratelimit.movie`  | Create content. `multipart/form-data`. Required: `overview`, `original_language`. EntityRef fields: `genres`, `casts`, `tags`, `mood_tags`, `studios` (format: `id:name,…`). File fields: `poster`, `cover`. Returns `201`. |
| `DELETE` | `/v1/a/content/{contentId}` | `content.Delete`       | `timeout`, `validate.movieId` | Permanently delete a content item.                                                                                                                                                                                          |
| `POST`   | `/v1/a/content/{contentId}` | `content.UploadAssets` | `validate.contentId`          | Upload video asset files for existing content. `multipart/form-data`. Required: `assetType` (`TRAILER`\|`TEASER`\|`CLIP`\|`PROMO`\|`BTS`), `videos` (repeatable file field). Returns `201`.                                 |

### People

| Method   | Path                              | Handler                    | Middlewares                    | Description                                                                                                                                                                                             |
| -------- | --------------------------------- | -------------------------- | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/v1/a/people/`                   | `person.ListAdmin`         | `timeout`                      | Paginated list of all person records with audit fields.                                                                                                                                                 |
| `GET`    | `/v1/a/people/{peopleId}`         | `person.GetAdmin`          | `timeout`, `validate.personId` | Full admin detail for a person.                                                                                                                                                                         |
| `POST`   | `/v1/a/people/`                   | `person.Create`            | `timeout`, `ratelimit.person`  | Create a person. `multipart/form-data`. Required: `name`, `roles`, `gender`, `career_status`. EntityRef fields: `tags`, `categories`, `specialties`. File fields: `profile`, `backdrop`. Returns `201`. |
| `DELETE` | `/v1/a/people/{peopleId}`         | `person.Delete`            | `timeout`, `validate.personId` | Permanently delete a person record.                                                                                                                                                                     |
| `GET`    | `/v1/a/people/{peopleId}/content` | `content.GetByPersonAdmin` | `timeout`, `validate.personId` | Full `Movie` objects for all content associated with the person (vs. minimal in consumer endpoint).                                                                                                     |

### Attributes

| Method   | Path                             | Handler               | Middlewares                       | Description                                                                                                                                                                  |
| -------- | -------------------------------- | --------------------- | --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/v1/a/attributes/`              | `attribute.ListAdmin` | `timeout`                         | Complete list of all attributes including inactive ones and audit fields.                                                                                                    |
| `GET`    | `/v1/a/attributes/{attributeId}` | `attribute.GetAdmin`  | `timeout`, `validate.attributeId` | Full admin detail for a single attribute.                                                                                                                                    |
| `POST`   | `/v1/a/attributes/`              | `attribute.Create`    | `timeout`                         | Create an attribute. `multipart/form-data`. Required: `name`, `attribute_type` (comma-separated: `TAG`\|`MOOD`\|`GENRE`\|`CATEGORY`\|`SPECIALITY`\|`STUDIO`). Returns `201`. |
| `DELETE` | `/v1/a/attributes/{attributeId}` | `attribute.Delete`    | `timeout`, `validate.attributeId` | Permanently delete an attribute.                                                                                                                                             |

### Encoder

| Method | Path                    | Handler          | Middlewares                 | Description                                                                                                                                                                                           |
| ------ | ----------------------- | ---------------- | --------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST` | `/v1/a/encoder/`        | `encoder.Create` | `ratelimit.encoder`         | Submit a video file for async HLS encoding. `multipart/form-data`. Required: `contentId`, `contentType` (`MOVIE`\|`TV`), `video` (file). Returns `202` with `EncoderJob` (`jobId`, `status: QUEUED`). |
| `GET`  | `/v1/a/encoder/{jobId}` | `encoder.Get`    | `timeout`, `validate.jobId` | Poll encoding job status and output. Status flow: `QUEUED` → `PROCESSING` → `COMPLETED` \| `COMPLETED_WITH_WARNINGS` \| `FAILED` \| `CANCELLED`.                                                      |

---

## How to extend

1. Add the handler under `app/internal/http/`.
2. Register it in [`app/internal/http/routes/v1.json`](../app/internal/http/routes/v1.json).
3. Update [`docs/OPENAPI.yaml`](OPENAPI.yaml) and run `make openapi-lint`.
4. Add the new route to the appropriate table above (method, path, handler, middlewares, description).
