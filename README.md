# Bi8s (Go)

A REST API built with Go, [chi](https://github.com/go-chi/chi), and DynamoDB on AWS for managing movies and persons (performers/content creators).

## Requirements

- Go 1.25+
- AWS account with DynamoDB tables (`bi8s-dev` for movies, `bi8s-person-dev` for persons)
- S3 bucket for storing images
- [air](https://github.com/air-verse/air) _(optional, for live reload during development)_

## Environment Variables

| Variable                     | Description                                       | Default                                                                                             |
| ---------------------------- | ------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `APP_ENV`                    | Runtime environment (`dev`, `prod`, etc.)         | `prod`                                                                                              |
| `DYNAMODB_MOVIE_TABLE`       | DynamoDB table name for movies                    | `bi8s-dev`                                                                                          |
| `DYNAMODB_PERSON_TABLE`      | DynamoDB table name for persons                   | `bi8s-person-dev`                                                                                   |
| `AWS_REGION`                 | AWS region                                        | `us-east-1`                                                                                         |
| `AWS_ACCESS_KEY_ID`          | AWS access key (optional, falls back to IAM role) | —                                                                                                   |
| `AWS_SECRET_ACCESS_KEY`      | AWS secret key (optional, falls back to IAM role) | —                                                                                                   |
| `S3_BUCKET`                  | S3 bucket used to store images                    | —                                                                                                   |
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

## API Endpoints

### Movies

| Method   | Path              | Description                                                       |
| -------- | ----------------- | ----------------------------------------------------------------- |
| `GET`    | `/v1/movies`      | List all movies (public fields only)                              |
| `GET`    | `/v1/movies/{id}` | Get a movie by ID (all fields except stats and audit)             |
| `POST`   | `/v1/movies`      | Create a movie with poster and cover upload (multipart/form-data) |
| `DELETE` | `/v1/movies/{id}` | Delete a movie by ID                                              |

### Persons

| Method   | Path                      | Description                                                            |
| -------- | ------------------------- | ---------------------------------------------------------------------- |
| `GET`    | `/v1/persons`             | List all persons                                                       |
| `GET`    | `/v1/persons/{id}`        | Get a person by ID                                                     |
| `POST`   | `/v1/persons`             | Create a person with profile and backdrop upload (multipart/form-data) |
| `DELETE` | `/v1/persons/{id}`        | Delete a person by ID                                                  |
| `GET`    | `/v1/persons/{id}/movies` | Get all movies where the person is in the cast                         |

### Discover

| Method | Path                         | Description                                            |
| ------ | ---------------------------- | ------------------------------------------------------ |
| `GET`  | `/v1/discover/{attributeId}` | Discover movies by attribute (genre, tag, or mood tag) |

### Attributes

| Method   | Path                  | Description               |
| -------- | --------------------- | ------------------------- |
| `GET`    | `/v1/attributes`      | List all attributes       |
| `GET`    | `/v1/attributes/{id}` | Get an attribute by ID    |
| `POST`   | `/v1/attributes`      | Create a new attribute    |
| `DELETE` | `/v1/attributes/{id}` | Delete an attribute by ID |

### Health

| Method | Path         | Description    |
| ------ | ------------ | -------------- |
| `GET`  | `/v1/health` | Liveness check |

## Project Structure

```text
cmd/
  api/          # Entry point — wires config, AWS clients, and the HTTP server
  migrate/      # Database migration runner (future use)
internal/
  app/          # Shared config struct and the Application container
  aws/          # AWS config loader and DynamoDB/S3 client setup
  domain/       # Domain types (FileUploadInput, etc.)
  env/          # Helpers for reading environment variables
  errs/         # Centralized error definitions and HTTP error handlers
  http/         # Router, middleware, handlers, and JSON utilities
  model/        # Domain models (Movie, Person, EntityRef, Audit, Stats)
  repository/   # DynamoDB data access layer
  response/     # Shared response envelope and JSON writers
  service/      # Business logic layer
  storage/      # S3 file upload implementation
  utils/        # Utility functions (ID generation, date helpers)
  validation/   # Request validation (struct, file, performer, date, age)
migrations/     # Migration files (future use)
docs/           # Project documentation
  architecture.md   — layered architecture and request flow
  api.md            — all endpoints with examples
  dynamodb.md       — table design and access patterns
  performance.md    — benchmark results and optimization
  todo.md           — bugs, features, and improvements
  adr/              — architectural decision records
scripts/        # Helper scripts
```

## Author

[Vikas Singh](https://xanderbilla.com)
