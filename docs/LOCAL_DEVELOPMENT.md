# Local development

Spin up the entire stack — API, Redis, OTel collector, Tempo, Loki,
Prometheus, Grafana, and a UI placeholder — with one command.

## Prerequisites

| Tool                  | Min. version | Why                                           |
| --------------------- | ------------ | --------------------------------------------- |
| Docker + Compose v2   | 24 / 2.20    | Run the local stack.                          |
| Go                    | 1.25         | Build/test the API outside Docker.            |
| Make                  | 4            | Drive everything via `make <target>`.         |
| `awscli` _(optional)_ | 2.x          | Seed local DynamoDB / S3 if you point at AWS. |

Install the developer tooling (golangci-lint, staticcheck, govulncheck, air):

```bash
make setup
```

## First-time setup

```bash
git clone https://github.com/xanderbilla/bi8s-go.git
cd bi8s-go
cp .env.example .env       # adjust credentials/region as needed
make docker-up             # builds + starts everything
```

The API is now reachable at <http://localhost:8080/v1/health>.
Swagger UI: <http://localhost:8080/v1/docs>.
Grafana: <http://localhost:3000> (admin/admin by default — change it).

## Common commands

| Command                 | What it does                                             |
| ----------------------- | -------------------------------------------------------- |
| `make docker-up`        | Build & start the local stack in the background.         |
| `make docker-down`      | Stop and remove containers (data volumes preserved).     |
| `make docker-logs`      | Tail logs for all services.                              |
| `make test-unit`        | Run unit tests with race detector (`app/internal/...`).  |
| `make test-integration` | Run integration tests against the local stack (`test/`). |
| `make coverage`         | Generate `app/coverage.html`.                            |
| `make openapi-validate` | Lint `docs/openapi.yaml`.                                |
| `make clean`            | Remove build artefacts and coverage files.               |

## Run the API outside Docker (faster iteration)

The Compose stack still provides the dependencies; only the API runs
locally with hot reload via [air](https://github.com/air-verse/air).

```bash
make docker-up SERVICES="redis otel-collector tempo loki prometheus grafana"
cd app && air
```

`air` config is committed at `app/.air.toml` and rebuilds on Go file
changes (≈300 ms turnaround).

## Working with seed data

The repo ships sample DynamoDB JSON dumps under `assets/data/` and media
under `assets/images/` and `assets/videos/`. To load them:

```bash
./scripts/seed.sh        # loads assets/data into the configured tables
```

## Useful URLs

| URL                                               | What                                               |
| ------------------------------------------------- | -------------------------------------------------- |
| <http://localhost:8080/v1/health>                 | API health                                         |
| <http://localhost:8080/v1/docs>                   | Swagger UI                                         |
| <http://localhost:8080/v1/openapi.yaml>           | Raw OpenAPI                                        |
| <http://localhost:3000>                           | Grafana                                            |
| <http://localhost:9090>                           | Prometheus                                         |
| <http://localhost:3200>                           | Tempo                                              |
| <http://localhost:3100>                           | Loki                                               |
| <http://localhost:8888> / <http://localhost:8889> | OTel collector (own metrics / Prometheus exporter) |

## Troubleshooting

- **API exits immediately with `APP_ENV must be one of...`** — your `.env`
  is missing or has a bad value. See [CONFIGURATION.md](CONFIGURATION.md).
- **`bind: address already in use`** — another process owns one of the
  Compose ports. Override with `API_PORT=8090 make docker-up`.
- **DynamoDB `ResourceNotFoundException`** — table not created yet. Either
  point at a real AWS account (`AWS_ACCESS_KEY_ID` etc.) or create the
  tables in DynamoDB Local (see [DATABASE.md](DATABASE.md)).
- **Grafana asks to change password** — log in once with `admin/admin` and
  set a real one before sharing the URL.
