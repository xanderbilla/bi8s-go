# Error handling

This document is a single-page reference for how errors flow through the
binary. The contract surface — JSON envelope, HTTP status mapping, and stable
error codes — is defined in [`app/internal/errs`](../app/internal/errs) and
[`app/internal/response`](../app/internal/response); treat those packages as
the source of truth.

## Sections

- The typed-error model (`errs.Error` with code, kind, cause)
- HTTP-status mapping (`response.WriteError`)
- Stable error codes (do **not** rename — they are part of the public API)
- Logging vs response: what gets logged, what the client sees
- Panics: the recovery middleware shape and OTel `error.type` attribute

## Conventions

- Wrap every fallible boundary call with `errs.Wrap` and a stable code.
- Never expose internal messages or stack traces in HTTP bodies.
- Validation errors return `400` with a `validation` kind and a field map.
- Upstream timeouts surface as `504`; rate-limit hits as `429` with
  `Retry-After`.

## See also

- [`API.md`](API.md) — response envelope + sample error payloads
- [`OBSERVABILITY.md`](OBSERVABILITY.md) — error metric labels + log fields
