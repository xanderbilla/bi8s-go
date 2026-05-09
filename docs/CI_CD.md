# CI / CD

This document is a map of the GitHub Actions workflows that gate every change
and ship every release. The workflow files in [`.github/workflows`](../.github/workflows)
are the source of truth; this guide explains how the jobs fit together and
which ones are blocking versus advisory.

## Workflows

- `ci.yml` — pull-request gate. Blocking jobs: `go-vet`, `go-test`,
  `golangci-lint`, `govulncheck`, `staticcheck`, `nginx-validate`,
  `compose-config`, `tofu` (matrix dev+prod), `tflint`. Advisory jobs:
  `gosec`, `openapi-lint`, `trivy-fs`.
- `docker-publish.yml` — build, scan (Trivy, advisory), SBOM (SPDX-JSON via
  Syft, retained 30 days), publish to ECR (`:latest`, `:<sha>`, `:vX.Y.Z`),
  deploy via SSM, post-deploy `/healthz` gate.
- `infra-deploy.yml` — OpenTofu plan/apply per environment. Per-env serialized
  via `concurrency.group = infra-deploy-${workflow}-${ref}`.
- `release.yml` — tag-driven release notes + version artifact promotion.

## Local parity

Run the same gates locally:

```sh
make lint test
cd infra/tofu && tflint --recursive --minimum-failure-severity=error
docker compose -f infra/docker/docker-compose.yml config -q
```

## See also

- [`RELEASING.md`](RELEASING.md) — version bump + tag flow
- [`DEPLOYMENT.md`](DEPLOYMENT.md) — runtime topology that CI feeds
