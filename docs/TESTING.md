# Testing

Tests are split by scope and isolated with Go build tags so each layer can
run independently in CI and locally.

## Layout

```
app/
  internal/...           # Unit tests live next to the code (*_test.go)
  cmd/api/               # Server lifecycle tests (server_shutdown_test.go)
test/
  go.mod                 # Separate module — keeps test deps out of app/
  integration/           # //go:build integration  — hit a running API
  e2e/                   # //go:build e2e          — full user flows
  fixtures/              # Reusable JSON / YAML inputs
  testdata/              # Golden files / large binary fixtures
```

`test/` is its own Go module so integration / e2e dependencies (HTTP
clients, container helpers, AWS mocks) never bleed into the production
`app/` module.

## Build tags

| Layer       | Tag           | When to run                                                        |
| ----------- | ------------- | ------------------------------------------------------------------ |
| Unit        | _(none)_      | Every save / every PR. Fast, hermetic, no network.                 |
| Integration | `integration` | Against a running stack (`make docker-up`) or in CI with services. |
| E2E         | `e2e`         | Against a deployed environment (smoke tests).                      |

Files without the right tag are simply skipped — they won't compile-fail
the default `go test ./...` run.

## Running tests

| Command                                  | Scope                                                                   |
| ---------------------------------------- | ----------------------------------------------------------------------- |
| `make test-unit`                         | `cd app && go test -race -count=1 -short ./internal/...`                |
| `make test-integration`                  | `cd test && go test -race -count=1 -tags=integration ./integration/...` |
| `make coverage`                          | Unit tests with HTML coverage in `app/coverage.html`.                   |
| `cd test && go test -tags=e2e ./e2e/...` | E2E smoke against `BI8S_E2E_BASE_URL`.                                  |

CI runs the unit suite with race + coverage on every push (see
`.github/workflows/ci.yml`).

## Environment variables for integration / e2e

| Variable                    | Default                 | Purpose                        |
| --------------------------- | ----------------------- | ------------------------------ |
| `BI8S_INTEGRATION_BASE_URL` | `http://localhost:8080` | Where integration tests point. |
| `BI8S_E2E_BASE_URL`         | _(unset → tests skip)_  | Where e2e smoke tests point.   |

If the target URL is unreachable, the tests call `t.Skipf(...)` rather
than failing — so a missing local stack doesn't break `go test`.

## Conventions

- **Table-driven tests** for handlers, parsers, and pure functions.
- **`httptest.NewRecorder` + `chi.NewRouter`** for handler-level coverage
  (see `health_handler_test.go`, `encoder_handler_test.go`).
- **Fuzz tests** for parsers (`parser_fuzz_test.go`); run with
  `go test -run=^$ -fuzz=Fuzz -fuzztime=30s ./internal/http/...`.
- **No global `init()` magic** — wire dependencies with constructors so
  tests can substitute fakes.
- **Race detector always on** in CI (`-race`).
- **Cleanup** — every test that opens a file/server uses `t.Cleanup(...)`
  (or `defer`) to release resources.

## Coverage targets

- ≥ 70 % for `internal/http`, `internal/response`, `internal/errs`,
  `internal/validation`, `internal/env`, `internal/ctxutil`.
- Encoder, repository, and AWS client packages are exercised primarily
  through integration tests (real DynamoDB + S3 behaviour matters more
  than mocked paths).

## Lint & static analysis

```bash
cd app
go fmt ./... && go vet ./...
golangci-lint run            # config: app/.golangci.yml
staticcheck ./...
govulncheck ./...
```

The same commands run in the `quality` Docker stage and in the
`golangci-lint` / `govulncheck` jobs of `ci.yml`.
