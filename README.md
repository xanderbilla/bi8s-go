# Bi8s (Go)

A REST API built with Go, chi, and DynamoDB on AWS.

## Requirements

- Go 1.25+
- AWS account with a DynamoDB table named `bi8s-dev`

## Environment Variables

| Variable                | Description                                       | Default     |
| ----------------------- | ------------------------------------------------- | ----------- |
| `ENV`                   | Runtime environment name                          | `prod`      |
| `DYNAMODB_TABLE`        | DynamoDB table name                               | `bi8s-dev`  |
| `AWS_REGION`            | AWS region                                        | `us-east-1` |
| `AWS_ACCESS_KEY_ID`     | AWS access key (optional, falls back to IAM role) | —           |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key (optional, falls back to IAM role) | —           |

## Running

```sh
go run ./cmd/api
```

With [direnv](https://direnv.net/), copy your credentials into `.envrc` and run `direnv allow`. The server starts on `:8080`.

## API

| Method | Path         | Description     |
| ------ | ------------ | --------------- |
| `GET`  | `/v1/health` | Liveness check  |
| `GET`  | `/v1/movies` | List all movies |

## Project Structure

```
cmd/api/        # Entry point — wires config, AWS, and HTTP server
internal/
  app/          # Shared config and application container
  aws/          # AWS config and DynamoDB client setup
  env/          # Environment variable helpers with fallbacks
  http/         # Router, middleware, handlers, and JSON utilities
  repository/   # DynamoDB data access layer
```

## Author

[Vikas Singh](https://xanderbilla.com)
