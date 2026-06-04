# API

Complete HTTP API reference for `bi8s-go`.

Machine-readable contract: [OPENAPI.yaml](OPENAPI.yaml)
Route manifest source of truth: [app/internal/http/routes/v1.json](../app/internal/http/routes/v1.json)

## Base URL and groups

All endpoints are under `/v1`.

- Consumer (public read): `/v1/c/*`
- Admin (internal/network restricted): `/v1/a/*`
- System/docs: `/v1/health`, `/v1/livez`, `/v1/readyz`, `/v1/openapi.yaml`, `/v1/docs`

## Response envelope

Every endpoint returns the same envelope shape.

```json
{
  "success": true,
  "status": 200,
  "message": "ok",
  "data": {},
  "error": null,
  "path": "/v1/c/search",
  "requestId": "01H...",
  "timestamp": "2026-06-02T00:00:00Z"
}
```

Error response example:

```json
{
  "success": false,
  "status": 404,
  "message": "content not found",
  "data": null,
  "error": {
    "type": "NOT_FOUND_ERROR",
    "code": "NOT_FOUND",
    "title": "content not found",
    "detail": "content not found",
    "userMessage": "content not found",
    "context": null
  },
  "path": "/v1/c/content/abc123",
  "requestId": "01H...",
  "timestamp": "2026-06-02T00:00:00Z"
}
```

## Common parameters

- `contentId`: path ID, pattern `^[A-Za-z0-9_-]+$`, max length 100
- `peopleId`: path ID, pattern `^[A-Za-z0-9_-]+$`, max length 100
- `attributeId`: path ID, pattern `^[A-Za-z0-9_-]+$`, max length 100
- `limit`: cursor pagination size, default 20, max 100
- `cursor`: opaque pagination token from previous response
- `page`/`pageSize`: page-number pagination (search/related), page starts at 1, `pageSize` max 100

## Error model by class

Possible `error.type` values:

- `VALIDATION_ERROR`
- `AUTH_ERROR`
- `NOT_FOUND_ERROR`
- `CONFLICT_ERROR`
- `RATE_LIMIT_ERROR`
- `CLIENT_ERROR`
- `SERVER_ERROR`
- `ERROR`

Typical `error.code` values seen across handlers/services:

- `BAD_REQUEST`
- `VALIDATION_FAILED`
- `NOT_FOUND`
- `CONFLICT`
- `RATE_LIMITED`
- `NOT_READY`
- `SERVICE_UNAVAILABLE`
- `INTERNAL_ERROR`
- `METHOD_NOT_ALLOWED`

## Endpoint reference

### System

#### GET /v1/health

Request:

- No body

Response `200` data:

- `env`: string
- `version`: string
- `commit`: string
- `checks`: object map `{ dependencyName: "up" | "down" }`

Possible errors:

- `503 SERVICE_UNAVAILABLE` when any dependency health check fails

#### GET /v1/livez

Request:

- No body

Response `200` data:

- `version`: string

Possible errors:

- none under normal operation

#### GET /v1/readyz

Request:

- No body

Response `200`:

- Same `data` payload as `/v1/health`

Possible errors:

- `503 NOT_READY` before bootstrap readiness flag is set
- `503 SERVICE_UNAVAILABLE` when downstream checks fail

#### GET /v1/openapi.yaml

Request:

- No body

Response `200`:

- Raw OpenAPI YAML

#### GET /v1/docs

Request:

- No body

Response `200`:

- Swagger UI HTML

### Consumer search and playback

#### GET /v1/c/search

Request query:

- `query` (required): non-empty string
- `in` (optional): `all|movie|tv|people` (default `all`)
- `sort` (optional): `recent|latest|alpha_asc|alpha_desc` (default `recent`)
- `page`, `pageSize` (optional shared fallback)
- `contentPage`, `contentPageSize` (optional content slice controls)
- `peoplePage`, `peoplePageSize` (optional people slice controls)

Response `200` data:

- `content.results`: array of `MoviePublicList`
- `content.count`, `content.page`, `content.pageSize`
- `people.results`: array of `SearchPersonResult`
- `people.count`, `people.page`, `people.pageSize`
- `warnings` (optional): array of `{scope, code, message}` for partial backend failures

Possible errors:

- `400 BAD_REQUEST` for missing `query`, invalid `in`, invalid `sort`, invalid/deep page window
- `500 INTERNAL_ERROR` when both search backends fail

#### GET /v1/c/content/{contentId}/related

Request:

- Path `contentId`
- Query `page`, `pageSize`

Response `200` data:

- `items`: `MoviePublicList[]`
- `total`: integer
- `page`: integer
- `size`: integer (current page result count)

Possible errors:

- `400 BAD_REQUEST`
- `500 INTERNAL_ERROR`

#### GET /v1/c/play/{contentType}/{contentId}

Request:

- Path `contentType`: `movie|tv`
- Path `contentId`

Response `200` data (`PlaybackInfo`):

- `durationSec`: number
- `streaming`: `{type, masterPlaylist}`
- `video`: `{defaultQuality, qualities[]}`
- `audio`: `{defaultTrackId, tracks[]}`
- `subtitles`: `{defaultTrackId, tracks[]}`
- `thumbnails`: `{items[]}`
- `preview`: `{durationSec, url}`
- `sprite`: `{image, vtt}`

Possible errors:

- `404 NOT_FOUND` when playback is missing or content type does not match encoder job

### Consumer content and people

#### GET /v1/c/content/{contentId}

Request:

- Path `contentId`

Response `200` data (`MoviePublicDetail`):

- `id`, `title`, `overview`
- `backdropPath`, `posterPath`
- `releaseDate`, `firstAirDate`
- `adult`, `contentRating`, `originalLanguage`
- `genres[]`, `casts[]`, `tags[]`, `moodTags[]`, `studios[]`
- `contentType`, `originCountry[]`
- `runtime`, `status`, `tagline`
- `assets[]`
- `stats`: `{totalViews,totalLikes,averageRating}`

Possible errors:

- `400 BAD_REQUEST`
- `404 NOT_FOUND`

#### GET /v1/c/banner

Request query:

- `type` (optional): `movie|tv`

Response `200` data (`BannerContent`):

- `id`, `backdropPath`, `title`, `overview`, `contentRating`, `assets[]`

Possible errors:

- `404 NOT_FOUND` when no eligible banner exists

#### GET /v1/c/discover

Request query:

- `type` (optional): `latest|recent|popular|trending` (default `latest`)
- `content` (optional): `movie|tv`
- `limit`, `cursor` (cursor pagination)
- `sort` (optional): `alpha_asc|alpha_desc`

Response `200` data:

- `items`: `MoviePublicList[]`
- `nextCursor`: string or omitted
- `count`: number of returned items

Possible errors:

- `400 BAD_REQUEST` for invalid pagination/sort inputs

#### GET /v1/c/people/{peopleId}

Request:

- Path `peopleId`

Response `200` data (`PersonPublicDetail`):

- `id`, `contentType`, `name`, `legalName`, `roles[]`, `stageName`
- `bio`, `birthDate`, `birthPlace`, `nationality`, `gender`
- `height`, `weight`, `verified`, `active`, `debutYear`, `careerStatus`
- `profilePath`, `backdropPath`, `aliases[]`
- `measurements`
- `tags[]`, `categories[]`, `specialties[]`
- `career`, `socialPresence[]`

Possible errors:

- `400 BAD_REQUEST`
- `404 NOT_FOUND`

#### GET /v1/c/people/{peopleId}/content

Request:

- Path `peopleId`
- Query `type` optional `movie|tv`
- Query `limit`, `cursor`

Response `200` data:

- `items`: minimal list entries `{id,title,backdropPath}`
- `nextCursor`
- `count`

Possible errors:

- `400 BAD_REQUEST`

#### GET /v1/c/attributes

Request query:

- `type` optional; accepts singular/plural style values, normalized in handler
- `sort` optional `alpha_asc|alpha_desc`

Response `200` data:

- array of `AttributePublicDetail` with fields:
  - `id`, `name`, `attributeType[]`, `logo`, `svg`, `contentType`, `active`

Possible errors:

- `400 BAD_REQUEST` for invalid `type`/`sort`

#### GET /v1/c/attributes/{id}

Request:

- Path `id`
- Query `content` optional `movie|tv`
- Query `limit`, `cursor`, optional `sort`

Response `200` data:

- `items`: minimal list entries `{id,title,backdropPath}`
- `nextCursor`
- `count`

Possible errors:

- `400 BAD_REQUEST`

### Admin content

#### GET /v1/a/content/

Request query:

- `q` optional free-text search (matches title, overview, tagline, cast, tags, genres, mood tags, studios)
- `sort` optional `recent|latest|alpha_asc|alpha_desc`
- `limit`, `cursor` (when `q`/`sort` is used, `cursor` is numeric offset)

Response `200` data:

- paged `Movie` objects (admin shape)

Possible errors:

- `400 BAD_REQUEST`

#### GET /v1/a/content/{contentId}

Request:

- Path `contentId`

Response `200` data:

- full admin `Movie` object (includes internal/admin fields)

Possible errors:

- `404 NOT_FOUND`

#### POST /v1/a/content/

Request body:

- `multipart/form-data`
- Required fields parsed by handler: `overview`, `original_language`
- Optional text fields: `id,title,release_date,first_air_date,adult,content_rating,content_type,runtime,status,tagline,visibility,origin_country`
- Optional EntityRef text fields (comma-separated `id:name`): `genres,casts,tags,mood_tags,studios`
- Optional files: `poster`, `cover`

Response `201` data:

- created admin `Movie` object

Possible errors:

- `400 BAD_REQUEST` / validation
- `404 NOT_FOUND` (invalid referenced entities)
- `409 CONFLICT` (duplicate/conditional conflicts)
- `429 RATE_LIMITED`

#### DELETE /v1/a/content/{contentId}

Request:

- Path `contentId`

Response `200` data:

- `null`

Possible errors:

- `404 NOT_FOUND`

#### POST /v1/a/content/{contentId}

Request body:

- `multipart/form-data`
- Required fields (exact lowercase names):
  - `contenttype`
  - `assettype` (`TRAILER|TEASER|CLIP|PROMO|BTS`)
  - `video` (single file)
- Optional:
  - `contentid` (overrides path `contentId` if provided)

Response `201` data:

- `{contentId, assetType, uploadedCount, paths[]}`

Possible errors:

- `400 BAD_REQUEST` for missing/wrong fields or multiple videos
- `404 NOT_FOUND`

#### DELETE /v1/a/content/{contentId}/assets/{assetType}/keys/{keyId}

Request:

- Path `contentId`
- Path `assetType` in `TRAILER|TEASER|CLIP|PROMO|BTS`
- Path `keyId`

Response `200` data:

- `{contentId, assetType, keyId}`

Possible errors:

- `400 BAD_REQUEST` invalid `assetType`
- `404 NOT_FOUND`

### Admin people

#### GET /v1/a/people/

Request query:

- `q` optional free-text search (matches name, legal/stage names, bio, aliases, roles, tags, categories, specialties)
- `sort` optional `recent|latest|alpha_asc|alpha_desc`
- `limit`, `cursor` (when `q`/`sort` is used, `cursor` is numeric offset)

Response `200` data:

- paged admin `Person` objects

Possible errors:

- `400 BAD_REQUEST`

#### GET /v1/a/people/{peopleId}

Request:

- Path `peopleId`

Response `200` data:

- full admin `Person` object

Possible errors:

- `404 NOT_FOUND`

#### POST /v1/a/people/

Request body:

- `multipart/form-data`
- Required: `name`, `roles`, `gender`, `career_status`
- Common optional fields:
  - `legal_name,stage_name,bio,birth_date,birth_place,nationality,height,weight,aliases,verified,active,debut_year`
  - `measurements_bust,measurements_waist,measurements_hips,measurements_unit,measurements_body_type,measurements_eye_color,measurements_hair_color`
  - `career_start_year,career_previous_profession,career_known_for`
  - `tags,categories,specialties` (EntityRef list as `id:name,...`)
  - `sourceMetadata_confidence,sourceMetadata_sources,sourceMetadata_notes`
  - indexed social fields: `social[0][platform_id]`, `social[0][username]`, `social[0][verified]`, `social[0][available]`, repeating indexes
- Optional files: `profile`, `backdrop`

Response `201` data:

- created admin `Person` object

Possible errors:

- `400 BAD_REQUEST` / validation
- `404 NOT_FOUND` for unknown social platform attributes
- `429 RATE_LIMITED`

#### DELETE /v1/a/people/{peopleId}

Request:

- Path `peopleId`

Response `200` data:

- `null`

Possible errors:

- `404 NOT_FOUND`

#### GET /v1/a/people/{peopleId}/content

Request:

- Path `peopleId`
- Query `type` optional `movie|tv`
- Query `limit`, `cursor`

Response `200` data:

- `items`: minimal entries `{id,title,backdropPath}`
- `nextCursor`
- `count`

Possible errors:

- `400 BAD_REQUEST`

### Admin attributes

#### GET /v1/a/attributes/

Request query:

- `q` optional free-text search (matches name, id, type, logo, svg)
- `type` optional `GENRE|TAG|MOOD|STUDIO|CATEGORY|SPECIALITY|SOCIAL|PLATFORM`
- `sort` optional `recent|latest|alpha_asc|alpha_desc`
- `limit`, `cursor` (optional numeric pagination in filter mode)

Response `200` data:

- default mode: list of full admin `Attribute` objects
- filter mode (`q`/`type`/`sort`/`limit`/`cursor` provided): `{items,count,total,nextCursor}`

### Admin unified search

#### GET /v1/a/search

Request query:

- `entity` optional `all|content|people|attributes` (default `all`)
- `q` optional free-text query
- `sort` optional `recent|latest|alpha_asc|alpha_desc`
- `attributeType` optional attribute type filter for attribute results
- `id` optional exact ID lookup for detail mode
- `limit`, `cursor` (numeric offset cursor)

Response `200` data:

- `query`, `entity`, `sort`, `limit`, `cursor`
- Optional sections by scope: `content`, `people`, `attributes`
- Each section shape: `{items,count,total,nextCursor}`
- Detail mode (`id` used): `detail: {entity,item}`

Possible errors:

- `400 BAD_REQUEST` for invalid entity/sort/cursor inputs
- `404 NOT_FOUND` for unresolved id lookup

#### GET /v1/a/attributes/{attributeId}

Request:

- Path `attributeId`

Response `200` data:

- full admin `Attribute`

Possible errors:

- `404 NOT_FOUND`

#### POST /v1/a/attributes/

Request body:

- `multipart/form-data`
- Required fields:
  - `name`
  - `attribute_type` (comma-separated list)
- Optional:
  - `logo`, `svg`, `active`

Allowed `attribute_type` values:

- `TAG`, `MOOD`, `GENRE`, `CATEGORY`, `SPECIALITY`, `STUDIO`, `SOCIAL`, `PLATFORM`

Response `201` data:

- created admin `Attribute`

Possible errors:

- `400 BAD_REQUEST` / validation
- `409 CONFLICT`

#### DELETE /v1/a/attributes/{attributeId}

Request:

- Path `attributeId`

Response `200` data:

- `null`

Possible errors:

- `404 NOT_FOUND`

### Admin system

#### POST /v1/a/reindex

Request:

- No body

Response `200` data:

- `people`: integer
- `content`: integer
- `joinTableEntries`: integer

Possible errors:

- `500 INTERNAL_ERROR`
