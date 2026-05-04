# Deployment

`bi8s-go` is deployed as a single Linux container to **EC2** (Amazon
Linux 2023) behind **NGINX**. Infrastructure is provisioned with
**OpenTofu**, and CI publishes images to **GHCR** on every push to `dev`.

## Pipeline overview

```
┌────────────────┐    push    ┌────────────────┐    OIDC    ┌────────────────┐
│ developer push │──────────► │ GitHub Actions │──────────► │   AWS (Tofu)   │
└────────────────┘            └───────┬────────┘            └───────┬────────┘
                                      │                             │
                                      ▼                             ▼
                              ┌────────────────┐            ┌────────────────┐
                              │   GHCR image   │   docker   │ EC2 (NGINX +   │
                              │ ghcr.io/.../api│◄───pull────│  bi8s-go)      │
                              └────────────────┘            └────────────────┘
```

## CI workflows

All under `.github/workflows/`:

| Workflow             | Trigger                     | What it does                                                                                                                 |
| -------------------- | --------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| `ci.yml`             | `push`, `pull_request`      | `go vet`, `go test -race -cover`, `golangci-lint`, `govulncheck`, OpenAPI lint (advisory).                                   |
| `docker-publish.yml` | `push` to `dev`, tags       | Multi-arch (`linux/amd64`, `linux/arm64`) build → push to `ghcr.io/xanderbilla/bi8s-go` with `:dev` and `:sha-<short>` tags. |
| `infra-deploy.yml`   | manual / `infra/**` changes | `tofu fmt -check`, `tofu validate`, `tofu plan`, optional `apply` (gated by environment approval).                           |

CI authenticates to AWS via **GitHub OIDC** (no long-lived keys). The
trust policy lives in `infra/tofu/modules/github-oidc/`.

## Tofu layout

```
infra/tofu/
  bootstrap/        # one-time S3 backend + DynamoDB lock table bootstrap
  global/           # provider/version pins shared across envs
  modules/
    vpc/            # VPC + public/private subnets + NAT
    ec2/            # API EC2 instance, ALB, AMI selection
    dynamodb/       # 4 tables + GSIs (provisioned in dev, on-demand in prod)
    s3/             # bi8s-storage bucket (versioned, encrypted)
    iam/            # roles for EC2 instance profile + GHA OIDC
    security-group/ # SG rules per tier
    github-oidc/    # OIDC provider + role for CI
  envs/
    _shared/        # locals/variables shared by dev and prod
    dev/            # dev-specific composition (calls modules)
    prod/           # prod-specific composition
```

## Bootstrapping a new account

```bash
# 1. Create the remote state bucket + lock table once per account.
make tofu-bootstrap

# 2. Plan + apply the dev environment.
make tofu-plan ENV=dev
make tofu-apply ENV=dev
```

`scripts/init-backend.sh` wires the local backend config to the bucket
created by step 1.

## Promoting to production

1. Merge to `dev` → CI publishes `:dev` and `:sha-<short>` images.
2. Tag a release: `git tag v0.2.0 && git push --tags`.
3. CI publishes `:v0.2.0` and `:latest`.
4. Update `prod` Tofu variable `app_image_tag` to `v0.2.0`.
5. `make tofu-plan ENV=prod` → review → `make tofu-apply ENV=prod`.

The EC2 user-data script (`infra/scripts/update-ec2-configs.sh`) pulls
the new image and rolls the container with zero downtime via NGINX
upstream switching.

## Runtime

| Component        | Image / package                     | Notes                                                                                                  |
| ---------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------ |
| API              | `ghcr.io/xanderbilla/bi8s-go:<tag>` | Runtime stage of `app/Dockerfile`, runs as UID 10001.                                                  |
| NGINX            | distro package                      | Config in `infra/docker/nginx.conf`. TLS via Let's Encrypt (`infra/scripts/setup-ssl-letsencrypt.sh`). |
| CloudWatch Agent | distro package                      | Installed by `infra/scripts/install-cloudwatch-agent.sh`.                                              |

## Health & probes

- ALB target group health check: `GET /v1/livez` (200 = healthy).
- NGINX upstream probe: `GET /v1/readyz` (used by zero-downtime swap).
- Application readiness includes DynamoDB + S3 + Redis (when configured).

See [RUNBOOK.md](RUNBOOK.md) for incident response and rollback steps.

## Releases

Releases are tracked in [`../CHANGELOG.md`](../CHANGELOG.md) using the
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format and
SemVer. The version stored in `VERSION` is read by the build pipeline
and surfaced as `BUILD_VERSION` to the running binary.
