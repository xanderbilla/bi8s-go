# TODO

A list of known issues, missing features, and ideas for improvement. Not everything here needs to be done — treat this as a backlog.

## Bugs / Issues

- `GET /v1/movies/{id}` returns `200 OK` with `null` data when a movie is not found.
  It should return `404 Not Found` with a clear error message instead.

- `DELETE /v1/movies/{id}` returns `500 Internal Server Error` when the movie does not exist.
  It should return `404 Not Found`.

## Missing Features

- `PUT /v1/movies/{id}` — update an existing movie's title or year.

- Input validation on `POST /v1/movies`:
  - `id` must not be empty
  - `title` must not be empty
  - `year` must be a positive number (e.g. between 1888 and the current year)

- Graceful shutdown — the server currently stops immediately when killed.
  It should finish handling in-flight requests before shutting down.
  See the `TODO` comment in `cmd/api/main.go`.

## Performance

- Replace `Scan` in `GetAll` with a paginated approach using `ExclusiveStartKey` and `LastEvaluatedKey`.
  Right now only the first 1MB of data is returned. See [dynamodb.md](dynamodb.md) for details.

- Add a DynamoDB Global Secondary Index (GSI) if you need to query movies by title or year.

## Developer Experience

- Add a `docker-compose.yml` that starts a local DynamoDB instance so you can develop without a real AWS account.

- Add a `Dockerfile` so the app can be built and run as a container.

- Add endpoint override support to `internal/aws/config.go` so you can point the app at `dynamodb-local`.

## Testing

- Add unit tests for the service layer. Because the repository is an interface, you can write a fake in-memory implementation and test the service without hitting DynamoDB at all.

- Add integration tests against `dynamodb-local` to test the repository layer end to end.

- Add HTTP handler tests using Go's `net/http/httptest` package.

## Observability

- Replace `middleware.Logger` (basic stdout logging) with structured logging using Go's built-in `slog` package. Structured logs are much easier to search in tools like CloudWatch or Datadog.

- Propagate the request ID (set by `middleware.RequestID`) through the service and repository layers so every log line for a single request shares the same ID.

- Add basic metrics (request count, error rate, latency) — even a simple `/v1/metrics` endpoint would help.
