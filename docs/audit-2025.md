# bi8s-go Production Readiness Audit

**Mode:** Read-only audit. No code changed.
**Constraint honored:** Public API stable — routes, request/response JSON shapes, and env var names treated as frozen. Every recommendation below is non-breaking unless explicitly flagged.
**Scope reviewed:** `app/cmd/api`, `app/internal/{app,aws,ctxutil,env,errs,http,logger,model,repository,response,service,storage,utils,validation}` and `app/internal/http/middleware/ratelimit`.
**Severity legend:** CRITICAL (data/availability risk now) · HIGH (will hit prod under load or specific input) · MEDIUM (real bug or design smell, low blast radius today) · LOW (cleanup, hygiene) · NIT (style/coupling).

---

## 1. Executive summary

The codebase is well-organized for its size. ADR-driven layering is real (handler → service → repository), context plumbing is consistent, slog with request-scoped IDs is wired throughout, graceful shutdown is implemented, secure headers + per-route rate limiting + CORS hardening are present, and the validation framework is custom but coherent. Tests exist for the highest-risk packages (encoder, http parsers, errs).

There are **two critical issues** that should be fixed before relying on this in production at any scale, **two high-priority issues** that will surface under realistic load or specific user requests, and a long tail of medium/low cleanup that affects maintainability and consistency rather than correctness.

| Severity | Count |
| -------- | ----- |
| CRITICAL | 1     |
| HIGH     | 2     |
| MEDIUM   | 12    |
| LOW      | 13    |
| NIT      | 3     |

---

## 2. CRITICAL findings

### C1 — `GenerateNumericID` produces only ~900K possible IDs; used as DynamoDB primary key

**File:** [app/internal/utils/id.go](app/internal/utils/id.go#L13-L18)
**Used by:** [app/internal/service/person.go](app/internal/service/person.go), [app/internal/service/attribute.go](app/internal/service/attribute.go)

```go
func GenerateNumericID() string {
    ts := time.Now().UnixMilli() % 100000
    c := atomic.AddUint64(&counter, 1) % 10
    id := ((ts*10)+int64(c))%900000 + 100000
    return strconv.FormatInt(id, 10)
}
```

**Problem.** The output is bounded to the integer range `[100000, 999999]` — only **900,000 distinct values**. By the birthday paradox, collisions become statistically likely (>50%) at roughly **1,100 records**, and start happening regularly within the first few hundred records inserted in the same millisecond bucket. `Person.Create` and `Attribute.Create` use `attribute_not_exists(id)` so a collision raises `ConditionalCheckFailedException`. With current error wiring (see M1) this surfaces to the client as either 404 or 409 on a perfectly valid create request. It also means **two different real entities can never share an ID without the second create failing silently to the user as "already exists"**.

**Fix.** Replace `GenerateNumericID` with `GenerateID` (UUIDv4 — what `Movie` already uses) or a ULID/KSUID for sortable IDs. This is **public-API-safe** because IDs are opaque strings in the response envelope; existing 6-digit IDs in DynamoDB continue to work.

**Blast radius.** Person and Attribute tables only. No retro-migration required.

---

## 3. HIGH findings

### H1 — `GetDiscoverContent` silently ignores `popular` and `trending`

**File:** [app/internal/repository/movie.go](app/internal/repository/movie.go#L347-L386)

```go
switch discoverType {
case "popular":
case "trending":
default:
    sortByReleaseDateDesc(movies)
}
```

**Problem.** The `popular` and `trending` cases are **empty bodies**. Items are returned in whatever order DynamoDB Scan happens to surface them — effectively random. Public API consumers calling `/v1/c/discover/popular` get arbitrary order with no error. Documentation (and OpenAPI) almost certainly implies a meaningful sort.

**Fix options (all public-API-safe):**

1. Implement a real signal (e.g., maintain `viewCount` / `lastViewedAt` and sort).
2. Until then, sort deterministically (release date or createdAt) and document the limitation.
3. Or have the handler reject these modes with `501 Not Implemented` until ready.

### H2 — Encoder graceful shutdown can deadlock

**File:** [app/internal/service/encoder.go](app/internal/service/encoder.go#L162-L175)

```go
s.wg.Add(1)
go func(...) {
    defer s.wg.Done()
    ...
    s.sem <- struct{}{}        // blocks; no ctx-cancel select
    defer func() { <-s.sem }()
    ...
}(...)
```

`Wait(ctx)` selects on `s.wg.Wait()` vs `ctx.Done()`. Caller [server.go](app/cmd/api/server.go) uses this to drain on SIGTERM. But if `ENCODER_MAX_CONCURRENT=3` and 10 jobs are queued, the 7 waiting goroutines block on the semaphore for up to `ENCODER_JOB_TIMEOUT_SECONDS` (default **1800s**) per slot ahead of them. SIGTERM cannot cancel them because the send is unguarded.

**Fix.**

```go
select {
case s.sem <- struct{}{}:
case <-ctx.Done():
    s.failJob(ctx, jobPtr, "SHUTDOWN", "service shutting down before slot acquired", "queue", "", ctx.Err())
    return
}
```

Then plumb the parent shutdown context through `Wait`. Public API unaffected; this only changes behavior on shutdown.

---

## 4. MEDIUM findings

### M1 — Two parallel error-classification paths in `errs`

**File:** [app/internal/errs/errors.go](app/internal/errs/errors.go#L131-L184)
`From()` (used by `Write`) and `classifyOr` + `safeUserMessage` (used by `BadRequestError/NotFoundError/ConflictError`) duplicate logic. Same error → different status depending on which helper a handler picked. Handlers compensate by manually `if errs.IsConditionalCheckFailed(err) → NotFoundError(...)`; otherwise `Write(...)` would have returned 409. Works in practice but is fragile and a recurring source of next-engineer confusion.
**Fix.** Single classifier (`From`); make `BadRequestError/NotFoundError/ConflictError` thin wrappers that just override the status when caller is _certain_, and document.

### M2 — Encoder shutdown hang (already covered as H2 above; see also `os.Remove` errors ignored, hardcoded `/tmp`).

### M3 — Inconsistent ID strategy across services

`MovieService` uses `GenerateID()` (UUID); `PersonService`/`AttributeService` use the broken `GenerateNumericID` (C1). Standardize on UUID for all entities.

### M4 — Dead validation code

[app/internal/http/validation_middleware.go](app/internal/http/validation_middleware.go) defines `ValidateID`, `ValidationError`, `ErrInvalidID` — **no callers**. Active path is `URLParamValidator`. Delete the dead code.

### M5 — Dead and risky branches in `parseEntityRefs`

[app/internal/http/movie_parser.go](app/internal/http/movie_parser.go) — for casts/genres/tags/etc, the `len(parts)==1` and `len(parts)==2` branches do the same thing (both ignore the optional name). The else-branch fabricates IDs via `GenerateNumericID()` and is unreachable from current callers. Risk: a future refactor enables the branch and silently invents primary-key IDs that point at nothing.

### M6 — `UploadFileStream` couples to hardcoded purpose prefixes; 8-char UUID truncation

[app/internal/storage/s3.go](app/internal/storage/s3.go) — branches on `movies/`/`tv/` prefix substrings. UUIDs are truncated to 8 chars (~32-bit collision space) for object keys. At scale (millions of objects per resource), collisions are realistic. Use full UUID or a longer prefix.

### M7 — Rate-limit cleanup runs inline under write lock

[app/internal/http/middleware/ratelimit/ratelimit.go](app/internal/http/middleware/ratelimit/ratelimit.go) — the goroutine ticker already cleans; `GetLimiter` may _also_ trigger cleanup while holding the write lock, serializing all new-IP creations across a cleanup pass under bursty traffic. Rely on the ticker only, or use a dedicated cleanup mutex.

### M8 — Person/Attribute model has `oneof=PERSON|ATTRIBUTE` validate tag, but service forces value

[app/internal/model/person.go](app/internal/model/person.go), [app/internal/model/attribute.go](app/internal/model/attribute.go). Validating the model directly outside the service path will fail on empty `ContentType`. Either drop the tag and validate in service, or set the value before validation.

### M9 — All public list endpoints use Scan + filter

Documented in ADR-002, re-flag at scale: pagination is in-memory; latency grows with table size. Add at minimum GSIs by `visibility#status#createdAt` for the hot listing endpoints.

### M10 — Unbounded goroutine fan-out per encoder job

[app/internal/service/encoder_pipeline.go](app/internal/service/encoder_pipeline.go) launches N video qualities + 2 audio + 5 thumbnails + preview + sprite **simultaneously** per job, each spawning ffmpeg. With 4K input and `ENCODER_MAX_CONCURRENT=3`, that's potentially 30+ ffmpeg processes. CPU contention and OOM risk on small/medium VMs. Add an inner worker pool sized by `ENCODER_FFMPEG_PARALLELISM` (default `runtime.GOMAXPROCS(0)`).

### M11 — Master-playlist upload silently swallows errors

[app/internal/service/encoder_pipeline.go](app/internal/service/encoder_pipeline.go) — nested `if err == nil { if err == nil { ... } }` means any failure to write or upload the master playlist disappears. Job is marked completed without a usable playlist.
**Fix.** Promote to `job.Errors`/`Warnings` and downgrade status accordingly.

### M12 — `uploadFile` passes empty bucket+region

[app/internal/service/encoder_publish.go](app/internal/service/encoder_publish.go) — relies on `UploadFileStream` defaults. Works today; an env-var typo on `S3_BUCKET` could silently land uploads in the wrong place. Pass explicit values from config.

---

## 5. LOW findings

| ID  | File                                                                                 | Issue                                                                                         |
| --- | ------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------- |
| L1  | [app/internal/http/router.go](app/internal/http/router.go)                           | `MovieHandler` constructed via struct literal; others use `NewXxxHandler`. Standardize.       |
| L2  | [app/internal/http/movie_handler.go](app/internal/http/movie_handler.go)             | `validAssetTypes` map should be `model.AssetType.IsValid()`.                                  |
| L3  | [app/internal/app/bootstrap.go](app/internal/app/bootstrap.go)                       | `RunStartupHealthChecks` iterates a map — non-deterministic log order.                        |
| L4  | [app/internal/app/config.go](app/internal/app/config.go)                             | `defaultCORSOrigins` declared but `CORS_ALLOWED_ORIGINS` read separately — double default.    |
| L5  | [app/internal/http/movie_parser.go](app/internal/http/movie_parser.go)               | `contentTypeFromQuery` defaults to `"all"` with no allow-list.                                |
| L6  | [app/internal/ctxutil/context.go](app/internal/ctxutil/context.go)                   | `ContextKey` is `string` — should be unexported `struct{}`-typed key to prevent collisions.   |
| L7  | [app/internal/model/](app/internal/model/movie.go)                                   | Mixed date typing: `model.Date` in some entities, `string` + `daterange` tag in others.       |
| L8  | [app/internal/validation/file.go](app/internal/validation/file.go)                   | Hardcoded `/tmp`; `ExtractFile` reads whole file into memory.                                 |
| L9  | [app/internal/service/encoder.go](app/internal/service/encoder.go)                   | Hardcoded `/tmp`; `os.Remove` errors ignored.                                                 |
| L10 | [app/internal/utils/ffmpeg.go](app/internal/utils/ffmpeg.go)                         | `limitedWriter.Write` returns `len(p)` instead of `n` (broken `io.Writer` contract; latent).  |
| L11 | [app/internal/http/json.go](app/internal/http/json.go)                               | Error message hardcodes "12 MB" — won't track `ConfigureLimits`.                              |
| L12 | [app/internal/service/encoder_pipeline.go](app/internal/service/encoder_pipeline.go) | `os.MkdirTemp("/tmp", ...)` hardcoded; use `os.TempDir()` or a config var.                    |
| L13 | [app/internal/errs/errors.go](app/internal/errs/errors.go)                           | Handlers manually pre-check `IsConditionalCheckFailed` → 404; works but is the symptom of M1. |

---

## 6. NIT

- **N1** [app/internal/http/middleware.go](app/internal/http/middleware.go) — `RequestLogger` imports `ratelimit.GetClientIP`. Move client-IP helper to a shared `httputil`.
- **N2** [app/internal/http/movie_handler.go](app/internal/http/movie_handler.go) — `GetBanner` returns `ErrContentNotFound` when no banner; deserves its own sentinel or `204 No Content`.
- **N3** [app/internal/http/encoder_handler.go](app/internal/http/encoder_handler.go) — inline `switch` for content type; consider `model.ContentType.Parse(string)`.

---

## 7. Production-readiness checklist (against the user's 13 review areas)

| #   | Area                        | Status         | Notes                                                                                                                                                                                          |
| --- | --------------------------- | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | Project structure           | ✅ Good        | Clear layered packages; ADRs present.                                                                                                                                                          |
| 2   | DRY                         | ⚠️ Mostly good | `errs` dual-classifier (M1); inline content-type switches (N3); duplicated branches in `parseEntityRefs` (M5).                                                                                 |
| 3   | Code quality / Go idioms    | ⚠️             | Dead code (M4); broken `io.Writer` contract (L10); error swallowing (M11).                                                                                                                     |
| 4   | Env / config                | ⚠️             | Typed env helpers are excellent; double-default CORS (L4); hardcoded `/tmp` everywhere (L8/L9/L12); hardcoded "12 MB" message (L11).                                                           |
| 5   | Standardized response/error | ⚠️             | Envelope is consistent; classifier duplication (M1) breaks the standard in subtle ways.                                                                                                        |
| 6   | Layered architecture        | ✅             | Clean per ADR-003.                                                                                                                                                                             |
| 7   | Validation / middleware     | ⚠️             | Custom validators are good; dead `ValidateID` (M4); model-vs-service coupling on ContentType (M8).                                                                                             |
| 8   | Swagger/OpenAPI             | ⚠️             | `openapi.yaml` exists; not verified in this audit to match `popular`/`trending` (H1).                                                                                                          |
| 9   | Testing                     | ⚠️             | Encoder + parsers + errs tested. Repos & handlers under-tested. No integration test against DynamoDB Local.                                                                                    |
| 10  | Logging / observability     | ✅             | slog JSON, request_id, expvar metrics. Could add OpenTelemetry traces.                                                                                                                         |
| 11  | Security                    | ⚠️             | SecureHeaders + CORS prod check + per-IP rate-limit are good. C1 enables enumeration of Person/Attribute IDs. No auth layer visible (likely intentional / out-of-scope, but worth confirming). |
| 12  | Documentation               | ✅             | `docs/` is unusually thorough for a Go service.                                                                                                                                                |
| 13  | Production readiness        | ⚠️             | Graceful shutdown exists but H2 can defeat it; encoder fan-out (M10) needs sizing; Scan+filter (M9) caps scale.                                                                                |

---

## 8. Recommended fix order (when you're ready to act)

**Wave 1 — must-fix before any prod traffic increase**

1. C1 — replace `GenerateNumericID` with `GenerateID` (UUID) for Person/Attribute.
2. H2 — guard `s.sem <-` with `ctx.Done()`; thread shutdown ctx into `Wait`.
3. H1 — implement or explicitly document `popular`/`trending`.

**Wave 2 — correctness & stability** 4. M1 + L13 — collapse error classifiers into one path. 5. M11 — surface master-playlist upload errors as job errors/warnings. 6. M10 — add inner worker pool to encoder pipeline. 7. M7 — remove inline cleanup in `GetLimiter`.

**Wave 3 — hygiene** 8. M4, M5 — delete dead code paths. 9. L1–L12 cleanups, batched by package. 10. M9 — design GSIs for hot listing endpoints (separate ADR + migration).

All of the above can be implemented without changing routes, request/response JSON, or env-var names.

---

_Audit performed read-only. No source code was modified._
