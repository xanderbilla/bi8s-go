# Performance

This document captures the latency and throughput targets the binary is
designed to meet, and the tools used to measure them. Numbers here are
_targets_, not guarantees; verified baselines live in the load-test artifacts
attached to the most recent CI run.

## Targets

- `/healthz` and `/readyz`: p99 < 50 ms.
- Cached read endpoints (`/v1/c/*` GETs after warmup): p95 < 100 ms.
- Cold reads (DynamoDB query + OpenSearch fan-out): p95 < 400 ms.
- Write/admin endpoints: p95 < 600 ms.

## Measurement

- Unit benchmarks: `go test -bench=. ./...` per package as needed.
- Integration timings: see [`test/integration`](../test/integration).
- End-to-end load: a `k6` or `vegeta` script driven from CI (target hosts
  the dev EC2 endpoint behind nginx).

## Knobs

- Connection pool sizes (HTTP, Redis, AWS SDK) — env-driven, see
  [`CONFIGURATION.md`](CONFIGURATION.md).
- Cache TTLs and warmup behaviour — see [`CACHING.md`](CACHING.md).
- DynamoDB capacity mode (`PAY_PER_REQUEST` by default; switch to
  provisioned + autoscaling under sustained high RPS).

## See also

- [`OBSERVABILITY.md`](OBSERVABILITY.md) — Grafana dashboards + alert rules
- [`RUNBOOK.md`](RUNBOOK.md) — on-call response when targets are missed
