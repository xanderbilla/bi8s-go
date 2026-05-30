# OpenAPI / Swagger UI

The API ships an OpenAPI 3.0 spec embedded into the binary and a Swagger UI
viewer that loads it.

## Endpoints

| Path                   | Description                                |
| ---------------------- | ------------------------------------------ |
| `GET /v1/openapi.yaml` | Raw OpenAPI 3.0 document (YAML)            |
| `GET /v1/docs`         | Swagger UI page (assets served from a CDN) |

The YAML source of truth lives at
[`app/internal/http/assets/openapi.yaml`](../app/internal/http/assets/openapi.yaml)
and a mirrored copy is kept at [`docs/openapi.yaml`](openapi.yaml) for
reviewers who want to read it without checking out the Go module.

## Coverage

The spec documents:

- The standard success and error envelope schemas.
- Every public consumer route (`/v1/c/*`).
- Every admin route (`/v1/a/*`). Admin routes are intentionally
  unauthenticated at the application layer; restrict them at the network
  perimeter.
- All standard error codes (see the README error code table).

## Updating the spec

1. Edit `app/internal/http/assets/openapi.yaml`.
2. Mirror it to `docs/openapi.yaml`:
   `cp app/internal/http/assets/openapi.yaml docs/openapi.yaml`
3. `cd app && go test ./internal/http -run TestServeOpenAPISpec`.
