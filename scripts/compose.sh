#!/usr/bin/env bash
# Thin wrapper around `docker compose` that always resolves build-time
# version/commit/date from the working tree before delegating. Use this for
# any local compose invocation so the api image always carries real ldflags
# values instead of the "dev"/"unknown" defaults baked into the Dockerfile.
#
# Usage: scripts/compose.sh -f docker-compose.local.yml up -d --build
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# VERSION file is the source of truth; fall back to "dev" if missing.
if [[ -z "${BUILD_VERSION:-}" ]]; then
  if [[ -f "${repo_root}/VERSION" ]]; then
    BUILD_VERSION="$(tr -d '[:space:]' < "${repo_root}/VERSION")"
  else
    BUILD_VERSION="dev"
  fi
fi

# git short SHA, with "-dirty" suffix when the working tree has unstaged changes.
if [[ -z "${BUILD_COMMIT:-}" ]]; then
  if BUILD_COMMIT="$(git -C "${repo_root}" rev-parse --short HEAD 2>/dev/null)"; then
    if ! git -C "${repo_root}" diff --quiet 2>/dev/null \
       || ! git -C "${repo_root}" diff --cached --quiet 2>/dev/null; then
      BUILD_COMMIT="${BUILD_COMMIT}-dirty"
    fi
  else
    BUILD_COMMIT="unknown"
  fi
fi

if [[ -z "${BUILD_DATE:-}" ]]; then
  BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi

export BUILD_VERSION BUILD_COMMIT BUILD_DATE

exec docker compose "$@"
