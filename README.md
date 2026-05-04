# Bi8s (Go)

A REST API built with Go, [chi](https://github.com/go-chi/chi), and DynamoDB on AWS for managing movies and persons (performers/content creators).

## Requirements

- Go 1.25+
- Docker & Docker Compose (for deployment)
- AWS account
- [air](https://github.com/air-verse/air) _(optional, for live reload during development)_

All environment variables with defaults and validation rules are documented in [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Local Development

```sh
# Run all quality checks (go fmt, go vet, go test -race, golangci-lint, staticcheck, govulncheck)
make test

# Build binary (runs make test first)
make build

# Run locally (runs make test first)
make run

# Run the API service inside the local Docker Compose stack
make run-docker
```

Server starts on `:8080`

## Quick Start

```sh
# View all available commands
make help

# Development workflow
make test
make build
make run
make run-docker
```

Full API reference is in [docs/api.md](docs/api.md).

## Project Structure

```text
app/
  cmd/api/            # Application entry point
  internal/
    ├── http/         # HTTP handlers and routing
    ├── service/      # Business logic
    ├── repository/   # Data access layer
    ├── model/        # Domain models
    └── ...
infra/                # Infrastructure as Code
  ├── tofu/           # OpenTofu/Terraform configs
  ├── docker/         # Docker & Nginx configs
  └── scripts/        # Deployment scripts
test/                 # Integration & e2e test module
docs/                 # Documentation (see docs/INDEX.md)
```

## Documentation

See [docs/INDEX.md](docs/INDEX.md) for the full documentation index.

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) before submitting a pull request.

## License

[MIT](LICENSE)

## Author

[Vikas Singh](https://xanderbilla.com)
