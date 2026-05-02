#!/usr/bin/env bash
# seed.sh — Seed DynamoDB tables and S3 bucket for a fresh environment.
#
# Usage:
#   bash scripts/seed.sh
#
# Environment variables (all have defaults):
#   API_URL        — Base URL to poll for health (default: https://api.xanderbilla.com)
#   S3_BUCKET      — Target S3 bucket (default: bi8s-storage-dev)
#   AWS_REGION     — AWS region (default: us-east-1)
#   SKIP_DYNAMO    — Set to "1" to skip DynamoDB seeding
#   SKIP_S3        — Set to "1" to skip S3 image uploads
#   SKIP_VIDEO     — Set to "1" to skip video upload
#   MAX_ATTEMPTS   — Health check retries before giving up (default: 60)
#   RETRY_INTERVAL — Seconds between health check retries (default: 10)
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
MANIFEST="$REPO_ROOT/assets/manifest.json"

API_URL="${API_URL:-https://api.xanderbilla.com}"
S3_BUCKET="${S3_BUCKET:-bi8s-storage-dev}"
AWS_REGION="${AWS_REGION:-us-east-1}"
SKIP_DYNAMO="${SKIP_DYNAMO:-0}"
SKIP_S3="${SKIP_S3:-0}"
SKIP_VIDEO="${SKIP_VIDEO:-0}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-60}"
RETRY_INTERVAL="${RETRY_INTERVAL:-10}"

log() { echo "[seed] $(date '+%H:%M:%S') $*"; }
err() { echo "[seed] $(date '+%H:%M:%S') ERROR: $*" >&2; }

# ─── 1. Health gate ──────────────────────────────────────────────────────────
log "Waiting for API at $API_URL/v1/health ..."
attempt=0
until curl -sf --max-time 5 "$API_URL/v1/health" > /dev/null 2>&1; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
    err "API did not become healthy after $(( MAX_ATTEMPTS * RETRY_INTERVAL ))s. Aborting."
    exit 1
  fi
  log "  attempt $attempt/$MAX_ATTEMPTS — not ready, retrying in ${RETRY_INTERVAL}s..."
  sleep "$RETRY_INTERVAL"
done
log "API is healthy."

# ─── 2. DynamoDB seed ────────────────────────────────────────────────────────
_seed_dynamo_table() {
  local table_name="$1"
  local filepath="$2"

  if [ ! -f "$filepath" ]; then
    log "  SKIP $table_name: backup file not found at $filepath"
    return 0
  fi

  local item_count
  item_count=$(jq '.Items | length' "$filepath")
  if [ "$item_count" -eq 0 ]; then
    log "  SKIP $table_name: no items in backup"
    return 0
  fi

  log "  $table_name: importing $item_count items..."
  local chunk offset
  chunk=0
  while [ $(( chunk * 25 )) -lt "$item_count" ]; do
    offset=$(( chunk * 25 ))
    local request_items
    request_items=$(jq -c --arg table "$table_name" --argjson offset "$offset" \
      '{($table): [.Items[$offset:($offset+25)][] | {PutRequest: {Item: .}}]}' \
      "$filepath")
    aws dynamodb batch-write-item \
      --request-items "$request_items" \
      --region "$AWS_REGION" \
      --output json > /dev/null
    chunk=$(( chunk + 1 ))
  done
  log "  OK $table_name: $item_count items written"
}

if [ "$SKIP_DYNAMO" = "0" ]; then
  if ! command -v aws > /dev/null 2>&1; then
    err "AWS CLI not found. Cannot seed DynamoDB."
    exit 1
  fi
  if ! command -v jq > /dev/null 2>&1; then
    err "jq not found. Cannot parse DynamoDB backup files."
    exit 1
  fi

  log "Seeding DynamoDB tables..."
  _project="${PROJECT_NAME:-bi8s}"
  _env="${APP_ENV:-dev}"
  _data_dir="$REPO_ROOT/assets/data"

  _seed_dynamo_table \
    "${DYNAMODB_MOVIE_TABLE:-${_project}-content-table-${_env}}" \
    "${_data_dir}/${_project}-content-table-${_env}.json"
  _seed_dynamo_table \
    "${DYNAMODB_PERSON_TABLE:-${_project}-person-table-${_env}}" \
    "${_data_dir}/${_project}-person-table-${_env}.json"
  _seed_dynamo_table \
    "${DYNAMODB_ATTRIBUTE_TABLE:-${_project}-attributes-table-${_env}}" \
    "${_data_dir}/${_project}-attributes-table-${_env}.json"
  _seed_dynamo_table \
    "${DYNAMODB_ENCODER_TABLE:-${_project}-video-table-${_env}}" \
    "${_data_dir}/${_project}-video-table-${_env}.json"

  log "DynamoDB seed complete."
else
  log "SKIP_DYNAMO=1 — skipping DynamoDB."
fi

# ─── 3. S3 image uploads ─────────────────────────────────────────────────────
if [ "$SKIP_S3" = "0" ]; then
  log "Uploading seed images to s3://$S3_BUCKET ..."

  if ! command -v aws > /dev/null 2>&1; then
    err "AWS CLI not found. Cannot upload to S3."
    exit 1
  fi

  if ! command -v jq > /dev/null 2>&1; then
    err "jq not found. Cannot parse manifest.json."
    exit 1
  fi

  total=$(jq '.images | length' "$MANIFEST")
  i=0
  while IFS= read -r entry; do
    local_path="$REPO_ROOT/$(echo "$entry" | jq -r '.local')"
    s3_key=$(echo "$entry" | jq -r '.s3_key')
    content_type=$(echo "$entry" | jq -r '.content_type')
    i=$(( i + 1 ))

    if [ ! -f "$local_path" ]; then
      log "  [$i/$total] SKIP (file not found): $local_path"
      continue
    fi

    # Check if already exists in S3 to keep idempotent
    if aws s3api head-object --bucket "$S3_BUCKET" --key "$s3_key" \
        --region "$AWS_REGION" > /dev/null 2>&1; then
      log "  [$i/$total] EXISTS  s3://$S3_BUCKET/$s3_key"
    else
      aws s3 cp "$local_path" "s3://$S3_BUCKET/$s3_key" \
        --content-type "$content_type" \
        --region "$AWS_REGION" \
        --no-progress
      log "  [$i/$total] UPLOAD  s3://$S3_BUCKET/$s3_key"
    fi
  done < <(jq -c '.images[]' "$MANIFEST")

  log "S3 image uploads complete."
else
  log "SKIP_S3=1 — skipping S3 images."
fi

# ─── 4. Video upload (optional) ──────────────────────────────────────────────
if [ "$SKIP_VIDEO" = "0" ]; then
  video_local="$REPO_ROOT/$(jq -r '.video.local' "$MANIFEST")"
  video_content_id="$(jq -r '.video.content_id' "$MANIFEST")"
  video_s3_key="videos/raw/${video_content_id}/sample.mp4"

  if [ -f "$video_local" ]; then
    if aws s3api head-object --bucket "$S3_BUCKET" --key "$video_s3_key" \
        --region "$AWS_REGION" > /dev/null 2>&1; then
      log "Video already exists at s3://$S3_BUCKET/$video_s3_key — skipping."
    else
      log "Uploading raw video to s3://$S3_BUCKET/$video_s3_key ..."
      aws s3 cp "$video_local" "s3://$S3_BUCKET/$video_s3_key" \
        --content-type "video/mp4" \
        --region "$AWS_REGION" \
        --no-progress
      log "Video upload complete."
    fi
  else
    log "Video file not found at $video_local — skipping."
  fi
else
  log "SKIP_VIDEO=1 — skipping video upload."
fi

log "Seed completed successfully."
