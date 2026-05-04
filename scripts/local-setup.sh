#!/usr/bin/env bash
# local-setup.sh — Create only the DynamoDB tables and S3 bucket needed for
# local development.  Safe to run multiple times (idempotent).
#
# Environment variables (all optional — defaults match docker-compose.local.yml):
#   AWS_REGION               — default: us-east-1
#   AWS_ACCESS_KEY_ID        — default: test   (for LocalStack)
#   AWS_SECRET_ACCESS_KEY    — default: test   (for LocalStack)
#   LOCALSTACK_ENDPOINT      — e.g. http://localhost:4566  (omit for real AWS)
#   PROJECT_NAME             — default: bi8s
#   APP_ENV                  — default: dev
#   DYNAMODB_MOVIE_TABLE
#   DYNAMODB_PERSON_TABLE
#   DYNAMODB_ATTRIBUTE_TABLE
#   DYNAMODB_ENCODER_TABLE
#   DYNAMODB_ENCODER_CONTENT_ID_INDEX
#   S3_BUCKET
#
# Usage:
#   ./scripts/local-setup.sh                          # real AWS

set -euo pipefail

# ── helpers ──────────────────────────────────────────────────────────────────
log()  { echo "[local-setup] $(date '+%H:%M:%S') $*"; }
err()  { echo "[local-setup] $(date '+%H:%M:%S') ERROR: $*" >&2; }
ok()   { echo "[local-setup] $(date '+%H:%M:%S')  ✓  $*"; }

# ── deps ─────────────────────────────────────────────────────────────────────
for _cmd in aws jq; do
  command -v "$_cmd" > /dev/null 2>&1 || { err "$_cmd is required but not found."; exit 1; }
done

# ── config ───────────────────────────────────────────────────────────────────
_project="${PROJECT_NAME:-bi8s}"
_env="${APP_ENV:-dev}"

REGION="${AWS_REGION:-us-east-1}"
ENDPOINT="${LOCALSTACK_ENDPOINT:-}"

# Export fake creds when targeting LocalStack so the CLI doesn't bail.
if [ -n "$ENDPOINT" ]; then
  export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
  export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
  export AWS_DEFAULT_REGION="$REGION"
fi

MOVIE_TABLE="${DYNAMODB_MOVIE_TABLE:-${_project}-content-table-${_env}}"
PERSON_TABLE="${DYNAMODB_PERSON_TABLE:-${_project}-person-table-${_env}}"
ATTRIBUTE_TABLE="${DYNAMODB_ATTRIBUTE_TABLE:-${_project}-attributes-table-${_env}}"
ENCODER_TABLE="${DYNAMODB_ENCODER_TABLE:-${_project}-video-table-${_env}}"
ENCODER_GSI="${DYNAMODB_ENCODER_CONTENT_ID_INDEX:-contentId-index}"
BUCKET="${S3_BUCKET:-${_project}-storage-${_env}}"

# Build endpoint flag (empty string when using real AWS).
_ep_flag=()
[ -n "$ENDPOINT" ] && _ep_flag=(--endpoint-url "$ENDPOINT")

# ── DynamoDB helpers ──────────────────────────────────────────────────────────

_table_exists() {
  aws dynamodb describe-table \
    --table-name "$1" \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --output json > /dev/null 2>&1
}

_create_simple_table() {
  local table="$1"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions AttributeName=id,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --output json > /dev/null
  aws dynamodb wait table-exists \
    --table-name "$table" \
    --region "$REGION" \
    "${_ep_flag[@]}"
  ok "Created: $table"
}

_create_encoder_table() {
  local table="$1"
  local gsi="$2"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table (with GSI $gsi)"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=id,AttributeType=S \
      AttributeName=contentId,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --global-secondary-indexes "[
      {
        \"IndexName\": \"$gsi\",
        \"KeySchema\": [{\"AttributeName\": \"contentId\", \"KeyType\": \"HASH\"}],
        \"Projection\": {\"ProjectionType\": \"ALL\"}
      }
    ]" \
    --billing-mode PAY_PER_REQUEST \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --output json > /dev/null
  aws dynamodb wait table-exists \
    --table-name "$table" \
    --region "$REGION" \
    "${_ep_flag[@]}"
  ok "Created: $table"
}

# ── S3 helpers ────────────────────────────────────────────────────────────────

_bucket_exists() {
  aws s3api head-bucket \
    --bucket "$1" \
    --region "$REGION" \
    "${_ep_flag[@]}" 2>/dev/null
}

_configure_bucket() {
  local bucket="$1"

  # 1. Disable block-public-access so the bucket policy can take effect.
  log "Configuring public-access block: $bucket"
  aws s3api put-public-access-block \
    --bucket "$bucket" \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --public-access-block-configuration \
      "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false" \
    --output json > /dev/null
  ok "Public-access block disabled: $bucket"

  # 2. Attach a bucket policy that allows anonymous GET (public read).
  log "Applying public-read bucket policy: $bucket"
  aws s3api put-bucket-policy \
    --bucket "$bucket" \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --policy "{
      \"Version\": \"2012-10-17\",
      \"Statement\": [{
        \"Sid\": \"PublicReadGetObject\",
        \"Effect\": \"Allow\",
        \"Principal\": \"*\",
        \"Action\": \"s3:GetObject\",
        \"Resource\": \"arn:aws:s3:::${bucket}/*\"
      }]
    }" \
    --output json > /dev/null
  ok "Public-read policy applied: $bucket"

  # 3. Set CORS so browser requests (local frontend / Grafana) are accepted.
  log "Applying CORS rules: $bucket"
  aws s3api put-bucket-cors \
    --bucket "$bucket" \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --cors-configuration '{
      "CORSRules": [{
        "AllowedHeaders": ["*"],
        "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
        "AllowedOrigins": [
          "http://localhost:3000",
          "http://localhost:8080",
          "http://localhost:8443",
          "https://localhost:8443",
          "http://127.0.0.1:3000",
          "http://127.0.0.1:8080",
          "http://127.0.0.1:8443",
          "https://127.0.0.1:8443"
        ],
        "ExposeHeaders": ["ETag"],
        "MaxAgeSeconds": 3000
      }]
    }' \
    --output json > /dev/null
  ok "CORS configured: $bucket"
}

_create_bucket() {
  local bucket="$1"
  if _bucket_exists "$bucket"; then
    ok "S3 bucket already exists: $bucket"
  else
    log "Creating S3 bucket: $bucket"
    if [ "$REGION" = "us-east-1" ]; then
      aws s3api create-bucket \
        --bucket "$bucket" \
        --region "$REGION" \
        "${_ep_flag[@]}" \
        --output json > /dev/null
    else
      aws s3api create-bucket \
        --bucket "$bucket" \
        --region "$REGION" \
        --create-bucket-configuration LocationConstraint="$REGION" \
        "${_ep_flag[@]}" \
        --output json > /dev/null
    fi
    ok "Created: s3://$bucket"
  fi
  # Always (re-)apply public-access, policy, and CORS — idempotent.
  _configure_bucket "$bucket"
}

# ── main ──────────────────────────────────────────────────────────────────────

log "=== Local dev setup ==="
log "Region   : $REGION"
if [ -n "$ENDPOINT" ]; then
  log "Endpoint : $ENDPOINT"
else
  log "Endpoint : AWS (real)"
fi
echo ""

log "--- DynamoDB tables ---"
_create_simple_table  "$MOVIE_TABLE"
_create_simple_table  "$PERSON_TABLE"
_create_simple_table  "$ATTRIBUTE_TABLE"
_create_encoder_table "$ENCODER_TABLE" "$ENCODER_GSI"

echo ""
log "--- S3 bucket ---"
_create_bucket "$BUCKET"

echo ""
log "=== Done — all local resources are ready ==="
