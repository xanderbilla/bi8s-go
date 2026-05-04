# Changelog

All notable changes to **bi8s-go** are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1).
- `CHANGELOG.md`.
- Documentation set under `docs/` (INDEX, ARCHITECTURE, API, CONFIGURATION,
  LOCAL_DEVELOPMENT, TESTING, DEPLOYMENT, AWS, DATABASE, STORAGE, SECURITY,
  OBSERVABILITY, RUNBOOK).
- Root `Makefile` targets: `setup`, `lint`, `test-unit`, `test-integration`,
  `coverage`, `openapi-validate`, `tofu-plan`, `tofu-apply`, `docker-up`,
  `docker-down`.
- Starter scaffolding (READMEs and placeholder fixtures) under
  `test/integration`, `test/e2e`, `test/fixtures`, `test/testdata`.

### Removed

- Makefile targets `docker-quality`, `compose-quality-local`,
  `compose-quality-prod`. Quality gates now run via `make test`,
  `make lint`, and the CI workflow.
- LocalStack usage example from `scripts/local-setup.sh` (LocalStack is
  not part of the supported local stack).

## [0.1.0]

Initial public version.

### Added

- Go HTTP API (`/v1/c/*` consumer, `/v1/a/*` admin) backed by DynamoDB and S3.
- Encoder pipeline (ffmpeg → HLS) with concurrency control and graceful drain.
- Standardized response envelope (`success`, `status`, `message`, `data`,
  `error`, `path`, `requestId`, `timestamp`).
- Centralized error types with stable error codes.
- Structured logging via `slog` (JSON in prod, text in dev).
- OpenTelemetry traces and metrics export to OTel Collector → Tempo /
  Prometheus; logs shipped via Promtail → Loki.
- Rate limiting with pluggable memory or Redis backend.
- Multi-stage Dockerfile (non-root UID 10001, ffmpeg, slim runtime).
- Local development stack (`docker-compose.local.yml`) including Grafana,
  Prometheus, Loki, Tempo, Promtail, MinIO, Redis, OTel Collector.
- Production stack (`infra/docker/docker-compose.yml`) with NGINX TLS edge.
- OpenTofu modules under `infra/tofu/` for VPC, EC2, DynamoDB, S3, IAM.
- GitHub Actions: `ci.yml`, `docker-publish.yml`, `infra-deploy.yml`.

[Unreleased]: https://github.com/xanderbilla/bi8s-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/xanderbilla/bi8s-go/releases/tag/v0.1.0
