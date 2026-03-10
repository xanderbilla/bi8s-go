# Bi8s (Go)

A REST API built with Go, [chi](https://github.com/go-chi/chi), and DynamoDB on AWS.

## Requirements

- Go 1.25+
- AWS account with a DynamoDB table (default name: `bi8s-dev`)
- [air](https://github.com/air-verse/air) _(optional, for live reload during development)_

## Environment Variables

| Variable                | Description                                       | Default     |
| ----------------------- | ------------------------------------------------- | ----------- |
| `ENV`                   | Runtime environment (`dev`, `prod`, etc.)         | `prod`      |
| `DYNAMODB_TABLE`        | DynamoDB table name                               | `bi8s-dev`  |
| `AWS_REGION`            | AWS region                                        | `us-east-1` |
| `AWS_ACCESS_KEY_ID`     | AWS access key (optional, falls back to IAM role) | —           |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key (optional, falls back to IAM role) | —           |

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

| Method   | Path              | Description          |
| -------- | ----------------- | -------------------- |
| `GET`    | `/v1/health`      | Liveness check       |
| `GET`    | `/v1/movies`      | List all movies      |
| `GET`    | `/v1/movies/{id}` | Get a movie by ID    |
| `POST`   | `/v1/movies`      | Create a new movie   |
| `DELETE` | `/v1/movies/{id}` | Delete a movie by ID |

## Project Structure

```
cmd/
  api/          # Entry point — wires config, AWS clients, and the HTTP server
  migrate/      # Database migration runner (future use)
internal/
  app/          # Shared config struct and the Application container passed to handlers
  aws/          # AWS config loader and DynamoDB client setup
  env/          # Helpers for reading environment variables with fallback defaults
  http/         # Router, middleware, request handlers, and JSON response utilities
  repository/   # DynamoDB data access — all reads and writes to the database live here
  service/      # Business logic layer between handlers and the repository
migrations/     # Migration files (future use)
docs/           # Project documentation
scripts/        # Helper scripts (build, deploy, etc.)
```

## Author

[Vikas Singh](https://xanderbilla.com)
