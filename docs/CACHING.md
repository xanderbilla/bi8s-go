# Caching

This document describes the layered caching model used by the binary: in-process
LRU caches, the Redis-backed shared cache, and HTTP response caching headers.
The actual TTLs and key shapes are environment-driven; do not hard-code them
here — read them from `.env.example` and the `app/internal/redis` package.

## Sections

- In-process caches (search results, attribute lookups)
- Redis cache (key prefixes, TTL envelope, invalidation)
- HTTP response cache headers (`Cache-Control`, `ETag`, conditional GETs)
- Cache stampede protection (singleflight) and negative-result caching
- Operational concerns (warmup, flush, observability counters)

## See also

- [`app/internal/redis`](../app/internal/redis) — client + helpers
- [`OBSERVABILITY.md`](OBSERVABILITY.md) — `cache_hits_total` / `cache_misses_total`
- [`PERFORMANCE.md`](PERFORMANCE.md) — latency budgets that depend on cache hit ratios
