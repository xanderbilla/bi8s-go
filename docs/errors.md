# Error Codes

All error responses use a uniform JSON envelope:

```json
{
  "success": false,
  "status": 400,
  "code": "VALIDATION_FAILED",
  "message": "Request validation failed",
  "details": [{ "field": "Title", "code": "required", "message": "..." }],
  "path": "/v1/movies",
  "request_id": "abc-123",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

The envelope intentionally has **no separate `error` field** — the
human-readable text lives in `message` and the stable machine-readable
identifier lives in `code`.

## Stable codes

| Code                  | HTTP | When                                                                                   |
| --------------------- | ---- | -------------------------------------------------------------------------------------- |
| `VALIDATION_FAILED`   | 400  | Request body fails struct validation. `details` is a list of `{field, code, message}`. |
| `BAD_REQUEST`         | 400  | Malformed request (parsing, content-type, missing fields not handled by validator).    |
| `UNAUTHORIZED`        | 401  | Missing or invalid bearer token on a protected route.                                  |
| `FORBIDDEN`           | 403  | Authenticated but not allowed.                                                         |
| `NOT_FOUND`           | 404  | Resource does not exist.                                                               |
| `METHOD_NOT_ALLOWED`  | 405  | The HTTP method is not supported on the target route.                                  |
| `CONFLICT`            | 409  | Optimistic concurrency / duplicate / state conflict.                                   |
| `RATE_LIMITED`        | 429  | Per-IP token bucket exhausted. `Retry-After` header is also set.                       |
| `INTERNAL_ERROR`      | 500  | Unexpected server error. The raw cause is logged but never returned to the client.     |
| `NOT_READY`           | 503  | `/v1/readyz` queried before the server flipped its readiness flag.                     |
| `SERVICE_UNAVAILABLE` | 503  | One or more dependency probes (`/v1/health`, `/v1/readyz`) failed.                     |

## Guarantees

- The `code` field is stable and safe to switch on in clients.
- Internal error messages are never leaked to clients on `INTERNAL_ERROR`.
- `request_id` matches the `X-Request-ID` response header for correlation.
- `details` is only populated for `VALIDATION_FAILED` (currently).

## `errs.From` mapping

Handlers use a single helper, `errs.Write(w, r, err)`, which routes through
`errs.From(err)` to pick the correct HTTP status and stable code. The current
mapping table:

| Source error                                                                                                        | Code             | HTTP  |
| ------------------------------------------------------------------------------------------------------------------- | ---------------- | ----- |
| `*errs.APIError`                                                                                                    | passes through   | as-is |
| `errs.ErrContentNotFound`, `errs.ErrNoEncodingFound`, `errs.ErrNoCompletedEncoding`, `errs.ErrPlaybackNotAvailable` | `NOT_FOUND`      | 404   |
| `errs.ErrAttributeNameTaken`                                                                                        | `CONFLICT`       | 409   |
| `errs.ErrFileEmpty`, `errs.ErrResultTooLarge`                                                                       | `BAD_REQUEST`    | 400   |
| `*errs.PerformerNotFoundError`, `*errs.AttributeNotFoundError`                                                      | `BAD_REQUEST`    | 400   |
| Uploader / AWS-config init errors                                                                                   | `INTERNAL_ERROR` | 500   |
| DynamoDB `ConditionalCheckFailedException` (via `errs.IsConditionalCheckFailed`)                                    | `CONFLICT`       | 409   |
| Throttling (`errs.IsThrottled`: `ProvisionedThroughputExceeded`, `RequestLimitExceeded`)                            | `RATE_LIMITED`   | 429   |
| Anything else                                                                                                       | `INTERNAL_ERROR` | 500   |

Delete handlers keep an explicit `IsConditionalCheckFailed` branch that maps
to `NOT_FOUND` instead of `CONFLICT`, because for delete semantics a missing
row is the more useful answer to the caller.
