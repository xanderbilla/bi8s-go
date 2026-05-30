#!/usr/bin/env bash
# local-setup.sh — Create the DynamoDB tables needed for local development.
# Storage uses B2 Blaze (configured via .env); no S3/LocalStack bucket is created.
# Safe to run multiple times (idempotent).
#
# Environment variables (all optional — defaults match docker-compose.dev.yml):
#   AWS_REGION               — default: us-east-1
#   AWS_ACCESS_KEY_ID        — default: test   (for LocalStack)
#   AWS_SECRET_ACCESS_KEY    — default: test   (for LocalStack)
#   LOCALSTACK_ENDPOINT      — e.g. http://localhost:4566  (omit for real AWS)
#   PROJECT_NAME             — default: bi8s
#   APP_ENV                  — default: dev
#   DYNAMODB_CONTENT_TABLE
#   DYNAMODB_PERSON_TABLE
#   DYNAMODB_ATTRIBUTE_TABLE
#   DYNAMODB_ENCODER_TABLE
#   DYNAMODB_ENCODER_CONTENT_ID_INDEX
#   DYNAMODB_CONTENT_CAST_TABLE
#   DYNAMODB_CONTENT_ATTRIBUTE_TABLE
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

CONTENT_TABLE="${DYNAMODB_CONTENT_TABLE:-${_project}-content-table-${_env}}"
PERSON_TABLE="${DYNAMODB_PERSON_TABLE:-${_project}-person-table-${_env}}"
ATTRIBUTE_TABLE="${DYNAMODB_ATTRIBUTE_TABLE:-${_project}-attributes-table-${_env}}"
ENCODER_TABLE="${DYNAMODB_ENCODER_TABLE:-${_project}-video-table-${_env}}"
ENCODER_GSI="${DYNAMODB_ENCODER_CONTENT_ID_INDEX:-contentId-index}"
CONTENT_CAST_TABLE="${DYNAMODB_CONTENT_CAST_TABLE:-${_project}-content-cast-table-${_env}}"
CONTENT_ATTRIBUTE_TABLE="${DYNAMODB_CONTENT_ATTRIBUTE_TABLE:-${_project}-content-attribute-table-${_env}}"
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

_create_content_table() {
  local table="$1"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table (with visibility GSIs)"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=id,AttributeType=S \
      AttributeName=visibility,AttributeType=S \
      AttributeName=createdAt,AttributeType=S \
      AttributeName=contentType,AttributeType=S \
      AttributeName=releaseDate,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --global-secondary-indexes '[
      {
        "IndexName": "visibility-createdAt-index",
        "KeySchema": [
          {"AttributeName": "visibility", "KeyType": "HASH"},
          {"AttributeName": "createdAt", "KeyType": "RANGE"}
        ],
        "Projection": {"ProjectionType": "ALL"}
      },
      {
        "IndexName": "visibility-contentType-index",
        "KeySchema": [
          {"AttributeName": "visibility", "KeyType": "HASH"},
          {"AttributeName": "contentType", "KeyType": "RANGE"}
        ],
        "Projection": {"ProjectionType": "ALL"}
      },
      {
        "IndexName": "visibility-releaseDate-index",
        "KeySchema": [
          {"AttributeName": "visibility", "KeyType": "HASH"},
          {"AttributeName": "releaseDate", "KeyType": "RANGE"}
        ],
        "Projection": {"ProjectionType": "ALL"}
      }
    ]' \
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

# Add visibility-releaseDate-index GSI to an existing content table if not present.
_add_releasedate_gsi_if_missing() {
  local table="$1"
  local gsi_count
  gsi_count=$(aws dynamodb describe-table \
    --table-name "$table" \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --output json 2>/dev/null \
    | jq '.Table.GlobalSecondaryIndexes // [] | map(select(.IndexName == "visibility-releaseDate-index")) | length')
  if [ "${gsi_count:-0}" -gt 0 ]; then
    ok "GSI already exists: visibility-releaseDate-index on $table"
    return
  fi
  log "Adding GSI visibility-releaseDate-index to $table"
  aws dynamodb update-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=visibility,AttributeType=S \
      AttributeName=releaseDate,AttributeType=S \
    --global-secondary-index-updates '[
      {
        "Create": {
          "IndexName": "visibility-releaseDate-index",
          "KeySchema": [
            {"AttributeName": "visibility", "KeyType": "HASH"},
            {"AttributeName": "releaseDate", "KeyType": "RANGE"}
          ],
          "Projection": {"ProjectionType": "ALL"}
        }
      }
    ]' \
    --region "$REGION" \
    "${_ep_flag[@]}" \
    --output json > /dev/null
  log "Waiting for GSI visibility-releaseDate-index to become ACTIVE on $table..."
  while true; do
    local gsi_status
    gsi_status=$(aws dynamodb describe-table \
      --table-name "$table" \
      --region "$REGION" \
      "${_ep_flag[@]}" \
      --output json \
      | jq -r '.Table.GlobalSecondaryIndexes // [] | map(select(.IndexName == "visibility-releaseDate-index")) | .[0].IndexStatus // "MISSING"')
    if [ "$gsi_status" = "ACTIVE" ]; then
      break
    fi
    log "  GSI status: $gsi_status, retrying in 2s..."
    sleep 2
  done
  ok "Added GSI: visibility-releaseDate-index on $table"
}

_create_attribute_table() {
  local table="$1"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table (with name-index GSI)"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=id,AttributeType=S \
      AttributeName=name,AttributeType=S \
    --key-schema AttributeName=id,KeyType=HASH \
    --global-secondary-indexes '[
      {
        "IndexName": "name-index",
        "KeySchema": [{"AttributeName": "name", "KeyType": "HASH"}],
        "Projection": {"ProjectionType": "ALL"}
      }
    ]' \
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

_create_content_cast_table() {
  local table="$1"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table (PK=personId, SK=contentId)"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=personId,AttributeType=S \
      AttributeName=contentId,AttributeType=S \
    --key-schema \
      AttributeName=personId,KeyType=HASH \
      AttributeName=contentId,KeyType=RANGE \
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

_create_content_attribute_table() {
  local table="$1"
  if _table_exists "$table"; then
    ok "DynamoDB table already exists: $table"
    return
  fi
  log "Creating DynamoDB table: $table (PK=attributeId, SK=contentId)"
  aws dynamodb create-table \
    --table-name "$table" \
    --attribute-definitions \
      AttributeName=attributeId,AttributeType=S \
      AttributeName=contentId,AttributeType=S \
    --key-schema \
      AttributeName=attributeId,KeyType=HASH \
      AttributeName=contentId,KeyType=RANGE \
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
_create_content_table      "$CONTENT_TABLE"
_add_releasedate_gsi_if_missing "$CONTENT_TABLE"
_create_simple_table       "$PERSON_TABLE"
_create_attribute_table    "$ATTRIBUTE_TABLE"
_create_encoder_table      "$ENCODER_TABLE" "$ENCODER_GSI"
_create_content_cast_table "$CONTENT_CAST_TABLE"
_create_content_attribute_table "$CONTENT_ATTRIBUTE_TABLE"


echo ""
log "=== Done — all local resources are ready ==="
