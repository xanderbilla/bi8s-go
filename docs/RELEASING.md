# Releasing

This document is the checklist for cutting a new release of the binary. The
release flow is fully automated by GitHub Actions ([`release.yml`](../.github/workflows/release.yml)
and [`docker-publish.yml`](../.github/workflows/docker-publish.yml)); a human
only needs to bump the version and push a tag.

## Steps

1. Update [`VERSION`](../VERSION) using semver (`MAJOR.MINOR.PATCH`).
2. Add a top entry to [`CHANGELOG.md`](../CHANGELOG.md) under the new version
   header. Use the conventional sections: `Added`, `Changed`, `Fixed`,
   `Security`, `Removed`.
3. Open a PR titled `release: vX.Y.Z`. Wait for CI green.
4. Merge to the default branch.
5. Tag: `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
6. The tag triggers `release.yml` (release notes from CHANGELOG) and
   `docker-publish.yml` (`:vX.Y.Z` image + SBOM artifact, retained 30 days).
7. Verify post-deploy `/healthz` is green (the publish workflow gates on it).

## Rollback

- ECR keeps the last 10 immutable image tags (`:<sha>`, `:vX.Y.Z`).
- Re-run `docker-publish.yml` with the prior tag, or SSH/SSM into the box and
  `docker compose pull && docker compose up -d` after pinning the previous tag
  in the compose env.

## See also

- [`CI_CD.md`](CI_CD.md) — workflow map
- [`DEPLOYMENT.md`](DEPLOYMENT.md) — runtime topology
- [`RUNBOOK.md`](RUNBOOK.md) — incident response during a bad release
