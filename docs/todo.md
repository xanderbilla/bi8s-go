# TODO

A list of known issues, missing features, and ideas for improvement.

## Bugs / Issues

- No critical API behavior bugs currently tracked.

## Missing Features

- `PUT /v1/movies/{id}` — update an existing movie's title or metadata.
- `PUT /v1/persons/{id}` — update an existing person's information.

- Graceful shutdown — the server currently stops immediately when killed.
  It should finish handling in-flight requests before shutting down.
  See the `TODO` comment in `cmd/api/main.go`.

## Performance

- Replace `Scan` in `GetAll` with a paginated approach using `ExclusiveStartKey` and `LastEvaluatedKey`.
  Right now only the first 1MB of data is returned. See [dynamodb.md](dynamodb.md) for details.

- Add a DynamoDB Global Secondary Index (GSI) if you need to query movies by title or year.

## Scalability

- `GET /v1/movies` uses a `Scan`, which reads the entire table on every request. As the table grows
  this gets slower and more expensive. Replace it with a `Query` using a partition key and add
  proper pagination so clients can request a specific page of results.

- No caching — every single request hits DynamoDB, even for data that barely ever changes.
  Consider an in-memory cache (e.g. a simple Go `sync.Map` with a TTL) or an external cache
  like Redis to serve repeated reads without going all the way to the database.

- No pagination — if there are 10,000 movies the response contains all of them at once (or just
  the first 1MB). Add `limit` and `cursor` query parameters so clients can page through results.

- No retry logic — under heavy load DynamoDB will start throttling requests and return errors.
  The AWS SDK supports automatic retries with exponential backoff, but it needs to be configured.
  Without this, a brief spike in traffic can cause a wave of 500 errors.

- Single instance — right now the app runs as one process. To handle more traffic you would run
  multiple copies behind a load balancer (e.g. AWS ALB). The app is stateless so this would work
  today, but there is no deployment setup for it yet.

## Testing

- Add unit tests for the service layer. Because the repository is an interface, you can write a fake in-memory implementation and test the service without hitting DynamoDB at all.

- Add integration tests against `dynamodb-local` to test the repository layer end to end.

- Add HTTP handler tests using Go's `net/http/httptest` package.

## Observability

- Replace `middleware.Logger` (basic stdout logging) with structured logging using Go's built-in `slog` package. Structured logs are much easier to search in tools like CloudWatch or Datadog.

- Propagate the request ID (set by `middleware.RequestID`) through the service and repository layers so every log line for a single request shares the same ID.

- Add basic metrics (request count, error rate, latency) — even a simple `/v1/metrics` endpoint would help.

- Add distributed tracing (e.g., AWS X-Ray) to track requests across services.

## Security

- Add rate limiting per IP address to prevent abuse.
- Add authentication/authorization (JWT tokens, API keys, etc.).
- Add input sanitization and validation for all endpoints.
- Implement HTTPS-only in production (already done via Nginx).
- Add security headers (already done via Nginx).

## Infrastructure

- Add auto-scaling for EC2 instances based on CPU/memory usage.
- Add CloudWatch alarms for monitoring.
- Add automated backups for DynamoDB tables.
- Add disaster recovery plan and documentation.
- Consider multi-region deployment for high availability.
