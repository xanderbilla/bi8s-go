# Bi8s (Go)

A REST API built with Go, [chi](https://github.com/go-chi/chi), and DynamoDB on AWS.

## Requirements

- Go 1.25+
- AWS account with a DynamoDB table (default name: `bi8s-dev`)
- [air](https://github.com/air-verse/air) _(optional, for live reload during development)_

## Environment Variables

| Variable                     | Description                                       | Default                                                                                             |
| ---------------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `ENV`                        | Runtime environment (`dev`, `prod`, etc.)         | `prod`                                                                                              |
| `DYNAMODB_TABLE`             | DynamoDB table name                               | `bi8s-dev`                                                                                          |
| `AWS_REGION`                 | AWS region                                        | `us-east-1`                                                                                         |
| `AWS_ACCESS_KEY_ID`          | AWS access key (optional, falls back to IAM role) | —                                                                                                   |
| `AWS_SECRET_ACCESS_KEY`      | AWS secret key (optional, falls back to IAM role) | —                                                                                                   |
| `S3_BUCKET`                  | S3 bucket used to store movie images              | —                                                                                                   |
| `S3_POSTER_PREFIX`           | Key prefix for image objects in S3                | `movies`                                                                                            |
| `CORS_ALLOWED_ORIGINS`       | Comma-separated list of allowed CORS origins      | `http://localhost:3000,http://localhost:8443,https://localhost:8443,http://13.217.109.221:8443,...` |
| `CORS_ALLOW_PRIVATE_NETWORK` | Allow private network preflight requests          | `true`                                                                                              |

> If `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are not set, the AWS SDK falls back to the default credential chain (IAM role, `~/.aws/credentials`, etc.).

## Running

**Standard:**

```sh
go run ./cmd/api
```

**With live reload (recommended for development):**

```sh
air
```

With [direnv](https://direnv.net/), copy your credentials into `.envrc` and run `direnv allow`. The server starts on `:8080`.

## API

| Method   | Path              | Description                                                       |
| -------- | ----------------- | ----------------------------------------------------------------- |
| `GET`    | `/v1/health`      | Liveness check                                                    |
| `GET`    | `/v1/movies`      | List all movies                                                   |
| `GET`    | `/v1/movies/{id}` | Get a movie by ID                                                 |
| `POST`   | `/v1/movies`      | Create a movie with poster and cover upload (multipart/form-data) |
| `DELETE` | `/v1/movies/{id}` | Delete a movie by ID                                              |

Error responses are returned as JSON (including router-level `404` and `405`).
Validation is enforced on `POST /v1/movies` before writes.

## Project Structure

```text
cmd/
  api/          # Entry point — wires config, AWS clients, and the HTTP server
  migrate/      # Database migration runner (future use)
internal/
  app/          # Shared config struct and the Application container passed to handlers
  aws/          # AWS config loader and DynamoDB client setup
  env/          # Helpers for reading environment variables with fallback defaults
  errs/         # Centralized HTTP error mapping + safe client error messages
  http/         # Router, middleware, request handlers, and JSON response utilities
  repository/   # DynamoDB data access — all reads and writes to the database live here
  response/     # Shared response envelope and JSON writers used across packages
  service/      # Business logic layer between handlers and the repository
  validation/   # go-playground/validator wrapper for request payload validation
migrations/     # Migration files (future use)
docs/           # Project documentation
  architecture.md   — how the 3 layers work and the full request flow
  api.md            — all endpoints with example requests and responses
  dynamodb.md       — table design, access patterns, known limitations
  performance.md    — benchmark results, what the latency means, and how to improve it
  todo.md           — bugs, missing features, and improvement ideas
  adr/              — records of key technical decisions and why they were made
scripts/        # Helper scripts (build, deploy, etc.)
```

## Author

[Vikas Singh](https://xanderbilla.com)
