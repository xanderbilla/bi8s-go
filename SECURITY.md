# Security Policy

## Supported Versions

Only the latest minor release line of `bi8s-go` receives security fixes.

| Version | Supported |
| ------- | --------- |
| 0.1.x   | Yes       |
| < 0.1   | No        |

## Reporting a Vulnerability

Do not open a public GitHub issue for security vulnerabilities.

Report privately by emailing `security@bi8s.local` (or open a private security
advisory on GitHub via the repository "Security" tab). Include:

- A clear description of the issue and impact
- Steps to reproduce, proof-of-concept, or affected endpoints
- Affected commit hash, branch, or release tag
- Your environment (OS, Go version, deployment target)

You will receive an acknowledgement within 3 business days. We aim to provide
a remediation plan within 14 days and ship a fix on the next patch release.

## Coordinated Disclosure

Please do not publicly disclose the issue until a fix has been released and we
have published an advisory. Credit will be given in the changelog and advisory
unless you request anonymity.

## Hardening Posture

This project follows the practices documented in [docs/SECURITY.md](docs/SECURITY.md):

- Strict request validation at the HTTP boundary
- Rate limiting (memory or Redis backend) on public routes
- Admin routes (`/v1/a/*`) network-protected at the reverse proxy layer
- TLS termination at the edge (Nginx + Let's Encrypt) in production
- IMDSv2-only EC2 metadata, least-privilege IAM via OpenTofu modules
- Multi-stage Docker build, distroless-style runtime, non-root UID 10001
- Dependency scanning via `govulncheck` and `go mod` policies in CI

## Out of Scope

- Vulnerabilities requiring a compromised host or operator-level access
- Theoretical issues without a working PoC
- Findings against forks, archived branches, or third-party deployments
