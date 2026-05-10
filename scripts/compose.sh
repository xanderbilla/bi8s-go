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

# ---------------------------------------------------------------------------
# ECR authentication
# Docker cannot pull private ECR images without a valid token. Credentials
# (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_REGION) come from .env via
# direnv or are already exported in the shell. We derive the registry URL from
# UI_IMAGE_NAME if set, otherwise fall back to the default image in the compose
# file. Login is skipped gracefully if `aws` CLI is not installed.
# ---------------------------------------------------------------------------
if command -v aws > /dev/null 2>&1; then
  # Resolve the ECR registry hostname from the image name or the default.
  _ecr_image="${UI_IMAGE_NAME:-929910138721.dkr.ecr.us-east-1.amazonaws.com/enternflix:efx-image-dev-latest}"
  _ecr_registry="$(echo "${_ecr_image}" | cut -d'/' -f1)"
  _ecr_region="${AWS_REGION:-us-east-1}"

  if [[ "${_ecr_registry}" == *.amazonaws.com ]]; then
    echo "[compose.sh] Logging in to ECR: ${_ecr_registry} (region: ${_ecr_region})"
    aws ecr get-login-password --region "${_ecr_region}" \
      | docker login --username AWS --password-stdin "${_ecr_registry}" \
      || echo "[compose.sh] WARN: ECR login failed — UI image pull may fail"
  fi
else
  echo "[compose.sh] WARN: 'aws' CLI not found — skipping ECR login (UI image pull may fail)"
fi

exec docker compose "$@"
