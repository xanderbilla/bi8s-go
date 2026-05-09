#!/usr/bin/env bash
# Generate a local-development TLS cert for the nginx container using mkcert.
# Output: infra/docker/nginx/ssl/live/{cert.crt,cert.key} (matches the prod
# layout used by infra/docker/nginx/conf.d/api.conf).
#
# Usage:   scripts/dev/gen-local-cert.sh [host ...]
# Default hosts: localhost 127.0.0.1 ::1 bi8s.local

set -euo pipefail

HOSTS=("$@")
if [[ ${#HOSTS[@]} -eq 0 ]]; then
  HOSTS=(localhost 127.0.0.1 ::1 bi8s.local)
fi

if ! command -v mkcert >/dev/null 2>&1; then
  cat >&2 <<'ERR'
mkcert is required. Install it first:
  macOS:  brew install mkcert nss
  Linux:  see https://github.com/FiloSottile/mkcert#installation
Then run `mkcert -install` once to trust the local CA.
ERR
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CERT_DIR="$REPO_ROOT/infra/docker/nginx/ssl/live"
mkdir -p "$CERT_DIR"

mkcert -cert-file "$CERT_DIR/cert.crt" -key-file "$CERT_DIR/cert.key" "${HOSTS[@]}"
chmod 644 "$CERT_DIR/cert.crt"
chmod 600 "$CERT_DIR/cert.key"

echo "wrote $CERT_DIR/cert.{crt,key} for hosts: ${HOSTS[*]}"
