# test/

End-to-end and integration tests for **bi8s-go** live here. Pure unit tests
co-locate with the source under `app/internal/...`.

| Folder         | Purpose                                                                            | Run with                           |
| -------------- | ---------------------------------------------------------------------------------- | ---------------------------------- |
| `integration/` | Wire real adapters (DynamoDB Local, MinIO, Redis) and exercise full handlers.      | `make test-integration`            |
| `e2e/`         | Black-box tests against a running stack (`docker-compose.local.yml`).              | `go test -tags=e2e ./test/e2e/...` |
| `fixtures/`    | Static request/response payloads, golden files, and seed data shared across tests. | n/a                                |
| `testdata/`    | Per-test inputs (binary blobs, sample HLS segments, image thumbnails).             | n/a                                |

## Conventions

- **Build tags** gate slow / external tests:
  - `//go:build integration` for `test/integration/...`
  - `//go:build e2e` for `test/e2e/...`
- **Environment**: integration tests assume the local stack is up
  (`make docker-up`). E2E tests assume the API is reachable at
  `http://localhost:8080` (override with `BI8S_E2E_BASE_URL`).
- **Fixtures**: prefer JSON fixtures under `fixtures/` over inlined literals
  to keep tests readable and diff-friendly.
- **Cleanup**: integration tests must clean up DynamoDB / S3 state they
  create (use `t.Cleanup`).
