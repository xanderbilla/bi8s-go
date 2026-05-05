# Bi8s (Go)

Bi8s is a REST API built with Go and chi, designed to manage movies and persons (performers and content creators). It provides structured APIs for metadata management, integrates with DynamoDB for scalable storage, and is designed to support streaming platforms with clean service layering, observability, and deployment-ready infrastructure.

## Requirements

* Go 1.25+
* Docker and Docker Compose
* AWS account
* air (optional, for live reload)

Environment variables, defaults, and validation rules are defined in `docs/CONFIGURATION.md`.

## Local Development

```sh
make test
make build
make run
make run-docker
```

Server runs on `:8080`.

## Quick Start

```sh
make help
make test
make build
make run
make run-docker
```

## Frontend

The corresponding frontend application (streaming UI) is available in the EnternFlix repository:

[https://github.com/xanderbilla/enternflix](https://github.com/xanderbilla/enternflix)
Ensure the frontend is configured to use this API for proper integration.

## Project Structure

```text
app/
  cmd/api/
  internal/
    http/
    service/
    repository/
    model/

infra/
  tofu/
  docker/
  scripts/

test/
docs/
```

## Documentation

Detailed documentation is available in the `docs/` directory, including API reference, architecture, configuration, and deployment guides.

## Contributing

Refer to `CONTRIBUTING.md`.

## License

MIT

## Author

Vikas Singh
[https://xanderbilla.com](https://xanderbilla.com)
