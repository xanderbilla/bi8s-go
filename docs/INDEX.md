# Documentation index

Welcome to the **bi8s-go** documentation set. Each guide below is a focused,
ground-truth reference for one slice of the system. If you're new, read in
this order: **ARCHITECTURE → LOCAL_DEVELOPMENT → CONFIGURATION → API**.

| Doc                                          | What it covers                                                      |
| -------------------------------------------- | ------------------------------------------------------------------- |
| [ARCHITECTURE.md](ARCHITECTURE.md)           | High-level component diagram, request lifecycle, package layout.    |
| [API.md](API.md)                             | Public REST surface, response envelope, error model, route table.   |
| [openapi.yaml](openapi.yaml)                 | Machine-readable OpenAPI 3.0 spec served at `GET /v1/openapi.yaml`. |
| [openapi.md](openapi.md)                     | Notes on the embedded spec and Swagger UI.                          |
| [CONFIGURATION.md](CONFIGURATION.md)         | Every env var the binary reads, defaults, validation rules.         |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Run the full stack locally with Docker Compose.                     |
| [TESTING.md](TESTING.md)                     | Unit / integration / e2e layout, build tags, coverage.              |
| [DEPLOYMENT.md](DEPLOYMENT.md)               | Build → publish → deploy pipeline (CI + Tofu + EC2).                |
| [AWS.md](AWS.md)                             | IAM model, regions, AWS resources the app touches.                  |
| [DATABASE.md](DATABASE.md)                   | DynamoDB tables, GSIs, access patterns.                             |
| [STORAGE.md](STORAGE.md)                     | S3 layout (uploads, HLS, Loki/Tempo backends).                      |
| [SECURITY.md](SECURITY.md)                   | Threat model, secure defaults, secrets handling, hardening.         |
| [OBSERVABILITY.md](OBSERVABILITY.md)         | Logs, metrics, traces, dashboards, alerts.                          |
| [RUNBOOK.md](RUNBOOK.md)                     | Operational playbooks for common incidents.                         |

## Conventions used in these docs

- **Code paths** are workspace-relative (e.g. `app/internal/http/router.go`).
- **Env vars** use the same names the binary reads (see `.env.example`).
- **Routes** are grouped by `/v1/c/*` (consumer/public) and `/v1/a/*` (admin).
- All examples assume the local stack is up: `make docker-up`.

See [`../CONTRIBUTING.md`](../CONTRIBUTING.md) for contribution rules and
[`../CODE_OF_CONDUCT.md`](../CODE_OF_CONDUCT.md) for community standards.
