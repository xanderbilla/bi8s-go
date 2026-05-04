# Security

This document captures the security posture of `bi8s-go` — what is
defended by default, what is the operator's responsibility, and how to
report a vulnerability. See also [`../SECURITY.md`](../SECURITY.md) for
the disclosure policy.

## Threat model (summary)

| Asset                        | Threat                  | Mitigation                                                                                 |
| ---------------------------- | ----------------------- | ------------------------------------------------------------------------------------------ |
| Catalog data (DynamoDB)      | Unauthorized writes     | Admin endpoints fronted by NGINX/auth in prod; IAM scoped per resource.                    |
| Source media (S3 `uploads/`) | Public exposure         | Block-public-access on; signed URLs only.                                                  |
| Secrets (`.env`, AWS keys)   | Leakage via logs / repo | `.env` git-ignored, scrubbed from logs (`internal/logger`), CI uses OIDC (no static keys). |
| Service availability         | DoS / abuse             | Global + per-route rate limits (memory or Redis); request body cap; per-request timeout.   |
| Telemetry pipeline           | Log poisoning           | OTel collector validates OTLP; no direct ingest from clients.                              |

## Secure defaults

| Layer           | Default                                                                                                                                  |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| TLS             | Terminated by NGINX with Let's Encrypt; HSTS 1y/preload set in app.                                                                      |
| CORS            | Explicit allow-list, no `*`; `https://` enforced in `prod`.                                                                              |
| Headers         | `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Cross-Origin-Resource-Policy: same-origin`. |
| Body limits     | 1 MiB JSON body cap (`HTTP_MAX_JSON_BYTES`).                                                                                             |
| Timeouts        | 60 s per request, 30 s per DynamoDB call, 1800 s per encode.                                                                             |
| Container       | Distroless-style Debian slim, runs as UID 10001 non-root, no shell capabilities relied on.                                               |
| Trusted proxies | Honoured only when `TRUSTED_PROXIES` is set; ignored otherwise.                                                                          |

## Authentication & authorization

The OSS surface ships with **no built-in authentication**. The expected
production deployment puts NGINX (with `auth_request` to a SSO/JWT
sidecar) or an API gateway in front of the API, restricting `/v1/a/*`
to authenticated administrators. Consumer routes (`/v1/c/*`) are
intentionally public.

Enabling auth inside the binary is a roadmap item. Until then, do not
expose `/v1/a/*` to the public internet without a gateway.

## Secrets management

- `.env` is git-ignored; **never commit** it.
- AWS credentials in production come from the EC2 instance profile —
  no static keys on the host.
- CI authenticates to AWS via GitHub OIDC.
- Grafana admin password must be changed from `admin/admin` on first
  boot.
- Loki/Tempo authentication tokens (when used) are loaded from env
  vars, never from disk.

## Dependency hygiene

| Tool                 | When       | Where                      |
| -------------------- | ---------- | -------------------------- |
| `go mod tidy`        | every PR   | dev workflow               |
| `govulncheck`        | every PR   | `.github/workflows/ci.yml` |
| `golangci-lint`      | every PR   | same                       |
| `staticcheck`        | every PR   | same                       |
| Dependabot           | weekly     | `.github/dependabot.yml`   |
| Trivy _(image scan)_ | on publish | `docker-publish.yml`       |

CVE findings break CI on `high` and `critical`.

## OWASP Top 10 (web) coverage

| Category                    | Coverage                                                                                              |
| --------------------------- | ----------------------------------------------------------------------------------------------------- |
| A01 Broken Access Control   | Admin/consumer route split; gateway enforcement in prod.                                              |
| A02 Cryptographic Failures  | TLS at NGINX; SSE on S3; no plaintext secrets in logs.                                                |
| A03 Injection               | DynamoDB SDK uses parametrized expressions; validator on every input; no string-concatenated queries. |
| A04 Insecure Design         | Single response envelope; centralized errors; no stacktrace leakage.                                  |
| A05 Misconfiguration        | `Config.Validate()` fails fast; secure headers; `CORS=*` rejected.                                    |
| A06 Vulnerable Components   | govulncheck + Dependabot + Trivy.                                                                     |
| A07 Identification/Auth     | Gateway-based; no in-app session storage.                                                             |
| A08 Software/Data Integrity | Signed image tags via GHCR; reproducible Docker build (`-trimpath`).                                  |
| A09 Logging/Monitoring      | Full OTel stack; access logs include request id.                                                      |
| A10 SSRF                    | App makes no user-controlled outbound HTTP calls.                                                     |

## Reporting a vulnerability

See [`../SECURITY.md`](../SECURITY.md). **Do not** open a public GitHub
issue for security findings.
