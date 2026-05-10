# Bi8s (Go)

Bi8s is a REST API built with Go and chi, designed to manage movies and persons (performers and content creators). It provides structured APIs for metadata management, integrates with DynamoDB for scalable storage, and is designed to support streaming platforms with clean service layering, observability, and deployment-ready infrastructure.

## Requirements

- Go 1.25+
- Docker and Docker Compose
- AWS account (DynamoDB, EC2, S3, VPC, Route53, ACM Cert, etc.)
- Grafana, Prometheus, Loki, OTel, Tempo
- Swagger
- air (optional, for live reload)

Environment variables, defaults, and validation rules are defined in `docs/CONFIGURATION.md`.

## Local Development

```sh
make help
make test
make build
make run
make run-docker
```

Server runs on `:8080`.

## Documentation

Detailed documentation is available in the [`docs`](https://github.com/xanderbilla/bi8s-go/tree/main/docs) directory, including API reference, architecture, configuration, deployment, and local development guides.

The corresponding frontend application (streaming UI) is available in the EnternFlix repository:

[https://github.com/xanderbilla/enternflix](https://github.com/xanderbilla/enternflix)

Ensure the frontend is configured to use this API for proper integration.

## Screenshot

<img width="1906" height="994" alt="image" src="https://github.com/user-attachments/assets/b900e990-acd0-4851-858b-97aa2501f575" />

## Contributing

Refer to [`CONTRIBUTING.md`](https://github.com/xanderbilla/bi8s-go/tree/main/CONTRIBUTING.md).

## License

MIT

## Author

Vikas Singh
[https://xanderbilla.com](https://xanderbilla.com)
