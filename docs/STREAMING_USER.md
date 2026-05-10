# Streaming User Experience: Gap Analysis & Resolutions

This document tracks UX gaps identified during session 3 of the bi8s-go health pass, focusing on the **Streaming User** persona — an end user browsing, searching, and watching content.

---

## Priority Picks (Implemented)

| #   | Gap                                 | Status                 | Change Summary                                                                                                                    |
| --- | ----------------------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Stats invisible to consumers        | ✅ Done                | Added `Stats ContentStats` to `ContentPublicDetail` struct; `toContentPublicDetail` now maps `m.Stats`                            |
| 2   | No fuzzy/autocomplete search        | ✅ Already implemented | `buildTextQuery` uses `"fuzziness": "AUTO"` in `multi_match` and `simple_query_string` with wildcard (`q*`)                       |
| 3   | No "More Like This" recommendations | ✅ Done                | New endpoint `GET /v1/c/content/{contentId}/related` backed by OpenSearch MLT query; see [MLT Endpoint](#more-like-this-endpoint) |

---

## All 11 UX Gaps

| #   | Gap                                           | Impact                                                                                                 | Resolution                                                                                                |
| --- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------- |
| 1   | `stats` absent from public content detail API | Consumer cannot display view/like/rating counts                                                        | **Fixed** — field added to model + mapper                                                                 |
| 2   | No fuzzy search                               | Typos or partial titles yield zero results                                                             | **Already done** — `fuzziness:AUTO` + wildcard                                                            |
| 3   | No recommendations / More Like This           | Dead-end after watching content                                                                        | **Fixed** — `/related` endpoint added                                                                     |
| 4   | MoodTags not indexed                          | Search misses mood-based queries ("feel-good", "dark")                                                 | **Fixed** — `MoodTags` added to `contentDoc`, index mapping, and search fields (`moodTags.name^2`)        |
| 5   | No pagination cursor on search                | Deep offset pagination (from=10000) rejected by OpenSearch                                             | Documented limit; `maxSearchFrom=10000` guard returns 400 with `page exceeds maximum`                     |
| 6   | Presigned playback URLs expire                | Client holds stale URL after token expiry                                                              | Presigned URL TTL is 15 min (configurable `PRESIGN_TTL`); client should re-request before playback        |
| 7   | No content-level error on missing asset       | 404 vs 500 ambiguity when asset key missing in S3                                                      | `errs.NewNotFound` is returned from storage layer; maps to 404 in response                                |
| 8   | Content list endpoint returns full doc        | `/v1/c/search` returns `MoviePublicList` (trimmed); full `ContentPublicDetail` only on single-item GET | No change needed                                                                                          |
| 9   | Rate limiting hits authenticated users        | Global rate limit applies to all IPs including CI/CDN                                                  | Rate limit is IP-scoped via `X-Forwarded-For`; trusted proxy config documented in `docs/CONFIGURATION.md` |
| 10  | SSE banner has no fallback                    | If Redis is unavailable, banner fetch fails silently                                                   | `sync.Map` in-process cache is used as fallback when Redis is unreachable                                 |
| 11  | Encoder concurrency test was flaky            | `panicMockFileUploader` race caused false failures in CI                                               | Fixed — mock now uses mutex; documented in `app/internal/encoder/`                                        |

---

## More Like This Endpoint

**Route:** `GET /v1/c/content/{contentId}/related`

**Query parameters:**
| Param | Default | Description |
|-------|---------|-------------|
| `page` | 1 | 1-based page number |
| `pageSize` | 20 | Results per page |

**Response shape:**

```json
{
  "message": "related content",
  "data": {
    "items": [
      /* []MoviePublicList */
    ],
    "total": 42,
    "page": 1,
    "size": 20
  }
}
```

**OpenSearch query:** Uses `more_like_this` across fields:

- `title`, `overview`, `genres.name`, `tags.name`, `moodTags.name`
- `min_term_freq: 1`, `min_doc_freq: 1`, `max_query_terms: 25`
- Sorted by `_score desc`

**When search is disabled** (`SEARCH_ENABLED=false`): returns `items: [], total: 0`.

---

## Deferred Items

These items were identified but deferred with documented rationale:

| Item                                        | Effort | Blocker / Rationale                                                                                                                                                      |
| ------------------------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Encoder real ffmpeg integration test        | Large  | Needs LocalStack + a real test video file seeded in a test bucket. Unit-level mock coverage is sufficient for now.                                                       |
| Secrets Manager via AWS SSM Parameter Store | Medium | Bootstrap phase-split needed — SSM must be seeded before app starts. `env.GetSecret()` is a clean seam; can swap in a single PR when ops is ready.                       |
| OpenSearch dedicated master nodes (HA)      | Medium | Cost: 3 dedicated master nodes. `dedicated_master_enabled = false` is hardcoded in `infra/tofu/modules/opensearch/main.tf`. Enable when cluster grows past 5 data nodes. |
| Negative-path E2E tests (4xx/5xx flows)     | Small  | Rate-limit flakiness in CI makes negative-path E2E unreliable. Unit tests cover all error paths; E2E should focus on happy paths.                                        |

---

## Related Files

- [app/internal/model/content.go](../app/internal/model/content.go) — `ContentPublicDetail`, `ContentStats`
- [app/internal/http/mappers.go](../app/internal/http/mappers.go) — `toContentPublicDetail`
- [app/internal/search/opensearch.go](../app/internal/search/opensearch.go) — `buildMLTBody`, `OpenSearchProvider.MoreLikeThis`
- [app/internal/search/provider.go](../app/internal/search/provider.go) — `Provider` interface, `NoopProvider`
- [app/internal/service/search.go](../app/internal/service/search.go) — `SearchService.MoreLikeThis`
- [app/internal/http/search_handler.go](../app/internal/http/search_handler.go) — `SearchHandler.MoreLikeThis`
- [app/internal/http/router.go](../app/internal/http/router.go) — route registration
