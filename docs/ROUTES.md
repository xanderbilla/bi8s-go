# Routes

Canonical route index for the current binary.

Source of truth: [app/internal/http/routes/v1.json](../app/internal/http/routes/v1.json)
Machine-readable schema: [OPENAPI.yaml](OPENAPI.yaml)
Detailed request/response/error docs: [API.md](API.md)

## System

| Method | Path               | Handler              | Notes |
| ------ | ------------------ | -------------------- | ----- |
| GET | /v1/health | health.HealthCheck | Deep health with dependency map |
| GET | /v1/livez | health.Liveness | Process liveness |
| GET | /v1/readyz | health.Readiness | Readiness + dependency checks |
| GET | /v1/openapi.yaml | docs.OpenAPISpec | Raw OpenAPI YAML |
| GET | /v1/docs | docs.SwaggerUI | Swagger UI |

## Consumer routes (/v1/c)

| Method | Path | Handler | Notes |
| ------ | ---- | ------- | ----- |
| GET | /v1/c/search | search.Search | Full-text search, independent content/people pagination |
| GET | /v1/c/attributes | attribute.ListConsumer | Active attributes, optional type/sort |
| GET | /v1/c/content/{contentId} | content.GetConsumer | Public content detail |
| GET | /v1/c/content/{contentId}/related | search.MoreLikeThis | Related content page |
| GET | /v1/c/people/{peopleId} | person.GetConsumer | Public person detail |
| GET | /v1/c/people/{peopleId}/content | content.GetByPersonConsumer | Minimal content list by person |
| GET | /v1/c/banner | content.GetBanner | Featured banner |
| GET | /v1/c/attributes/{id} | content.GetByAttribute | Minimal content list by attribute |
| GET | /v1/c/discover | content.GetDiscover | Discover feed |
| GET | /v1/c/play/{contentType}/{contentId} | content.GetPlayback | Playback URLs for finished encoder job |

## Admin routes (/v1/a)

| Method | Path | Handler | Notes |
| ------ | ---- | ------- | ----- |
| POST | /v1/a/content/{contentId} | content.UploadAssets | Multipart video upload (`contenttype`,`assettype`,`video`) |
| DELETE | /v1/a/content/{contentId}/assets/{assetType}/keys/{keyId} | content.DeleteAssetKey | Delete single stored asset key |
| GET | /v1/a/content/ | content.ListAdmin | Paged admin content list |
| GET | /v1/a/content/{contentId} | content.GetAdmin | Admin content detail |
| POST | /v1/a/content/ | content.Create | Multipart content create |
| DELETE | /v1/a/content/{contentId} | content.Delete | Delete content |
| GET | /v1/a/people/ | person.ListAdmin | Paged admin people list |
| GET | /v1/a/people/{peopleId} | person.GetAdmin | Admin person detail |
| POST | /v1/a/people/ | person.Create | Multipart person create |
| DELETE | /v1/a/people/{peopleId} | person.Delete | Delete person |
| GET | /v1/a/people/{peopleId}/content | content.GetByPersonAdmin | Minimal content list by person |
| GET | /v1/a/attributes/ | attribute.ListAdmin | Admin attributes |
| GET | /v1/a/attributes/{attributeId} | attribute.GetAdmin | Admin attribute detail |
| POST | /v1/a/attributes/ | attribute.Create | Multipart attribute create |
| DELETE | /v1/a/attributes/{attributeId} | attribute.Delete | Delete attribute |
| POST | /v1/a/reindex | reindex.Trigger | Full reindex operation |

## Notes

- Admin routes are guarded by network-local middleware and should stay behind perimeter controls.
- Validation middleware enforces path parameter shape (`contentId`, `peopleId`, `attributeId`, etc.).
- Rate-limiting middleware is applied to admin write routes for content and people.
