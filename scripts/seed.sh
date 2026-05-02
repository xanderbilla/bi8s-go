#!/usr/bin/env bash
# seed.sh — Wipe existing data, then seed via API + S3, then create encoder job.
#
# Phases (in order):
#   1. Wipe (DynamoDB tables + S3 bucket)
#   2. Health gate (wait for API)
#   3. Seed via API (attributes → persons → movies)
#   4. Upload images + raw video to S3
#   5. Create encoder job via API
#
# Usage:
#   bash scripts/seed.sh
#
# Environment variables (all have defaults):
#   API_URL        — Base URL of the API (default: https://api.xanderbilla.com)
#   S3_BUCKET      — Target S3 bucket (default: bi8s-storage-dev)
#   AWS_REGION     — AWS region (default: us-east-1)
#   SKIP_WIPE      — Set to "1" to skip the wipe phase
#   SKIP_DYNAMO    — Set to "1" to skip DynamoDB seeding
#   SKIP_S3        — Set to "1" to skip S3 uploads
#   SKIP_ENCODER   — Set to "1" to skip encoder job creation
#   MAX_ATTEMPTS   — Health check retries before giving up (default: 60)
#   RETRY_INTERVAL — Seconds between health check retries (default: 10)
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"
MANIFEST="$REPO_ROOT/assets/manifest.json"

API_URL="${API_URL:-https://api.xanderbilla.com}"
S3_BUCKET="${S3_BUCKET:-bi8s-storage-dev}"
AWS_REGION="${AWS_REGION:-us-east-1}"
SKIP_WIPE="${SKIP_WIPE:-0}"
SKIP_DYNAMO="${SKIP_DYNAMO:-0}"
SKIP_S3="${SKIP_S3:-0}"
SKIP_ENCODER="${SKIP_ENCODER:-0}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-60}"
RETRY_INTERVAL="${RETRY_INTERVAL:-10}"

log() { echo "[seed] $(date '+%H:%M:%S') $*"; }
err() { echo "[seed] $(date '+%H:%M:%S') ERROR: $*" >&2; }

# ─── Preflight checks ─────────────────────────────────────────────────────────
if ! command -v aws > /dev/null 2>&1; then
  err "AWS CLI not found."
  exit 1
fi
if ! command -v jq > /dev/null 2>&1; then
  err "jq not found."
  exit 1
fi
if ! command -v curl > /dev/null 2>&1; then
  err "curl not found."
  exit 1
fi

_project="${PROJECT_NAME:-bi8s}"
_env="${APP_ENV:-dev}"
_data_dir="$REPO_ROOT/assets/data"

# ─── 1. Wipe DynamoDB ─────────────────────────────────────────────────────────
_wipe_dynamo_table() {
  local table_name="$1"
  local total_deleted=0
  local start_key=""

  while :; do
    local scan_args=(
      --table-name "$table_name"
      --region "$AWS_REGION"
      --projection-expression "id"
      --output json
    )
    if [ -n "$start_key" ]; then
      scan_args+=(--exclusive-start-key "$start_key")
    fi

    local scan_result
    scan_result=$(aws dynamodb scan "${scan_args[@]}")

    local items count
    items=$(echo "$scan_result" | jq '.Items')
    count=$(echo "$items" | jq 'length')

    if [ "$count" -gt 0 ]; then
      local chunk=0
      while [ $(( chunk * 25 )) -lt "$count" ]; do
        local offset=$(( chunk * 25 ))
        local request_items
        request_items=$(echo "$items" | jq -c \
          --arg table "$table_name" \
          --argjson offset "$offset" \
          '{($table): [.[$offset:($offset+25)][] | {DeleteRequest: {Key: {id: .id}}}]}')
        aws dynamodb batch-write-item \
          --request-items "$request_items" \
          --region "$AWS_REGION" \
          --output json > /dev/null
        chunk=$(( chunk + 1 ))
      done
      total_deleted=$(( total_deleted + count ))
    fi

    local next_key
    next_key=$(echo "$scan_result" | jq -rc '.LastEvaluatedKey // ""')
    if [ -z "$next_key" ] || [ "$next_key" = "null" ]; then
      break
    fi
    start_key="$next_key"
  done

  log "  WIPED $table_name: $total_deleted items deleted"
}

if [ "$SKIP_WIPE" = "0" ]; then
  log "──── PHASE 1: Wipe ─────────────────────────────────────────────────────────"
  log "Wiping DynamoDB tables..."
  _wipe_dynamo_table "${DYNAMODB_MOVIE_TABLE:-${_project}-content-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_PERSON_TABLE:-${_project}-person-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_ATTRIBUTE_TABLE:-${_project}-attributes-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_ENCODER_TABLE:-${_project}-video-table-${_env}}"
  log "DynamoDB wipe complete."

  log "Wiping S3 bucket s3://$S3_BUCKET ..."
  aws s3 rm "s3://$S3_BUCKET" --recursive --region "$AWS_REGION" 2>&1 | grep -v "^$" || true
  log "S3 wipe complete."
else
  log "SKIP_WIPE=1 — skipping wipe phase."
fi

# ─── 2. Health gate ───────────────────────────────────────────────────────────
log "──── PHASE 2: Health gate ──────────────────────────────────────────────────"
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

# ─── 3. Seed via API ──────────────────────────────────────────────────────────
# Generic multipart POST helper — usage: _api_post /v1/a/endpoint -F "key=val" ...
_api_post() {
  local endpoint="$1"; shift
  curl -sf --max-time 30 -X POST "${API_URL}${endpoint}" "$@"
}

if [ "$SKIP_DYNAMO" = "0" ]; then
  log "──── PHASE 3: Seed via API ─────────────────────────────────────────────────"

  # Maps: old seed ID → new API-assigned ID (needed because attribute/person parsers
  # do not accept an explicit id field — the service auto-generates UUIDs).
  declare -A _attr_id
  declare -A _person_id

  _create_attribute() {
    local old_id="$1" name="$2" attr_type="$3"
    local resp new_id
    resp=$(_api_post /v1/a/attributes \
      -F "name=${name}" \
      -F "attribute_type=${attr_type}") || { err "Failed to create attribute: ${name}"; exit 1; }
    new_id=$(echo "$resp" | jq -r '.data.id')
    _attr_id["$old_id"]="$new_id"
    log "  ATTR  ${name} (${attr_type}) → ${new_id}"
  }

  _create_person() {
    local old_id="$1"; shift
    local resp new_id
    resp=$(_api_post /v1/a/people "$@") || { err "Failed to create person (old_id=${old_id})"; exit 1; }
    new_id=$(echo "$resp" | jq -r '.data.id')
    _person_id["$old_id"]="$new_id"
    log "  PERSON old=${old_id} → ${new_id}"
  }

  # ── 3a. Attributes ──────────────────────────────────────────────────────────
  log "Creating attributes..."
  _create_attribute "126716" "Action"              "GENRE"
  _create_attribute "176697" "Drama"               "GENRE"
  _create_attribute "277219" "Fetish"              "CATEGORY,SPECIALITY"
  _create_attribute "225828" "Steamy"              "MOOD,TAG"
  _create_attribute "331270" "Vivid Entertainment" "STUDIO"

  # ── 3b. Persons ─────────────────────────────────────────────────────────────
  log "Creating persons..."
  _create_person "648032" \
    -F "name=Angelina Jolie" \
    -F "stage_name=AJ" \
    -F "roles=PERFORMER,CONTENT_CREATOR" \
    -F "bio=Angelina Jolie is an American actress, filmmaker, and humanitarian. Known for iconic roles in Lara Croft: Tomb Raider, Mr. & Mrs. Smith, Maleficent, and Eternals, she is one of the most celebrated and influential figures in Hollywood. She won an Academy Award for Best Supporting Actress for Girl, Interrupted." \
    -F "birth_date=1975-06-04" \
    -F "birth_place=Los Angeles, California" \
    -F "nationality=American" \
    -F "gender=Female" \
    -F "height=173" \
    -F "active=true" \
    -F "debut_year=1993" \
    -F "career_status=Active" \
    -F "measurements_bust=36" \
    -F "measurements_waist=27" \
    -F "measurements_hips=36" \
    -F "measurements_unit=inches" \
    -F "measurements_body_type=Slim" \
    -F "measurements_eye_color=Green" \
    -F "measurements_hair_color=Black" \
    -F "tags=${_attr_id[225828]}:Steamy" \
    -F "categories=${_attr_id[277219]}:Fetish" \
    -F "specialties=${_attr_id[277219]}:Fetish"

  _create_person "542131" \
    -F "name=Robert Downey Jr." \
    -F "stage_name=RDJ" \
    -F "roles=PERFORMER,CONTENT_CREATOR" \
    -F "bio=Robert John Downey Jr. is an American actor renowned for his portrayal of Tony Stark / Iron Man in the Marvel Cinematic Universe. With a career spanning over five decades, he is celebrated as one of the most talented and versatile actors of his generation, earning two Academy Award nominations and a BAFTA." \
    -F "birth_date=1965-04-04" \
    -F "birth_place=Manhattan, New York City, NY" \
    -F "nationality=American" \
    -F "gender=Male" \
    -F "height=174" \
    -F "active=true" \
    -F "debut_year=1970" \
    -F "career_status=Active" \
    -F "measurements_unit=cm" \
    -F "measurements_body_type=Athletic" \
    -F "measurements_eye_color=Brown" \
    -F "measurements_hair_color=Brown" \
    -F "tags=${_attr_id[225828]}:Steamy" \
    -F "categories=${_attr_id[277219]}:Fetish" \
    -F "specialties=${_attr_id[277219]}:Fetish"

  # ── 3c. Movies ──────────────────────────────────────────────────────────────
  log "Creating movies..."
  _resp=""

  _resp=$(_api_post /v1/a/movies \
    -F "id=cb844b75-3547-4c88-a480-41a6912c34a8" \
    -F "title=Eternals" \
    -F "overview=The Eternals, a race of immortal beings with superhuman powers who have secretly lived on Earth for thousands of years, reunite to battle the monstrous Deviants and uncover a startling secret about their own existence." \
    -F "content_type=MOVIE" \
    -F "status=RELEASED" \
    -F "visibility=PUBLIC" \
    -F "adult=true" \
    -F "content_rating=18_PLUS" \
    -F "original_language=en" \
    -F "runtime=156" \
    -F "release_date=2021-11-05" \
    -F "tagline=In the beginning..." \
    -F "origin_country=US" \
    -F "genres=${_attr_id[126716]}:Action" \
    -F "casts=${_person_id[648032]}:Angelina Jolie,${_person_id[542131]}:Robert Downey Jr." \
    -F "tags=${_attr_id[225828]}:Steamy" \
    -F "mood_tags=${_attr_id[225828]}:Steamy" \
    -F "studios=${_attr_id[331270]}:Vivid Entertainment") || { err "Failed to create movie: Eternals"; exit 1; }
  log "  MOVIE Eternals → $(echo "$_resp" | jq -r '.data.id')"

  _resp=$(_api_post /v1/a/movies \
    -F "id=2517125d-ec6f-4cdf-b2b9-ca9db5c79709" \
    -F "title=Marvel Anime: Iron Man" \
    -F "overview=Tony Stark travels to Japan to build a new arc reactor and unveil Iron Man Dio. When the villainous ZODIAC steals the suit, Tony dons the original armor to stop them in an epic clash across the neon-lit streets of Tokyo." \
    -F "content_type=MOVIE" \
    -F "status=ENDED" \
    -F "visibility=PUBLIC" \
    -F "adult=false" \
    -F "content_rating=18_PLUS" \
    -F "original_language=ja" \
    -F "runtime=25" \
    -F "release_date=2010-10-01" \
    -F "tagline=Forged in fire. Reborn in iron." \
    -F "origin_country=US" \
    -F "genres=${_attr_id[126716]}:Action,${_attr_id[176697]}:Drama" \
    -F "casts=${_person_id[542131]}:Robert Downey Jr.,${_person_id[648032]}:Angelina Jolie" \
    -F "tags=${_attr_id[225828]}:Steamy" \
    -F "mood_tags=${_attr_id[225828]}:Steamy" \
    -F "studios=${_attr_id[331270]}:Vivid Entertainment") || { err "Failed to create movie: Marvel Anime Iron Man"; exit 1; }
  log "  MOVIE Marvel Anime: Iron Man → $(echo "$_resp" | jq -r '.data.id')"

  log "API seed complete."
else
  log "SKIP_DYNAMO=1 — skipping API seed."
fi

# ─── 4. S3 uploads (images + raw video) ──────────────────────────────────────
if [ "$SKIP_S3" = "0" ]; then
  log "──── PHASE 4: S3 uploads ───────────────────────────────────────────────────"

  # Images
  log "Uploading seed images to s3://$S3_BUCKET ..."
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

    aws s3 cp "$local_path" "s3://$S3_BUCKET/$s3_key" \
      --content-type "$content_type" \
      --region "$AWS_REGION" \
      --no-progress
    log "  [$i/$total] UPLOAD  s3://$S3_BUCKET/$s3_key"
  done < <(jq -c '.images[]' "$MANIFEST")
  log "Image uploads complete."

  # Raw video — uploaded directly to S3 as encoder input
  video_local="$REPO_ROOT/$(jq -r '.video.local' "$MANIFEST")"
  video_content_id="$(jq -r '.video.content_id' "$MANIFEST")"
  video_s3_key="videos/raw/${video_content_id}/sample.mp4"

  if [ -f "$video_local" ]; then
    log "Uploading raw video to s3://$S3_BUCKET/$video_s3_key ..."
    aws s3 cp "$video_local" "s3://$S3_BUCKET/$video_s3_key" \
      --content-type "video/mp4" \
      --region "$AWS_REGION" \
      --no-progress
    log "Raw video upload complete."
  else
    log "WARNING: Raw video not found at $video_local — skipping raw upload."
  fi
else
  log "SKIP_S3=1 — skipping S3 uploads."
fi

# ─── 5. Create encoder job ────────────────────────────────────────────────────
if [ "$SKIP_ENCODER" = "0" ]; then
  log "──── PHASE 5: Create encoder job ───────────────────────────────────────────"

  video_local="$REPO_ROOT/$(jq -r '.video.local' "$MANIFEST")"
  video_content_id="$(jq -r '.video.content_id' "$MANIFEST")"

  if [ ! -f "$video_local" ]; then
    err "Video file not found at $video_local — cannot create encoder job."
    exit 1
  fi

  log "Submitting encoder job: contentId=$video_content_id contentType=MOVIE ..."
  response=$(curl -sf --max-time 300 -X POST "$API_URL/v1/a/encoder" \
    -F "contentId=$video_content_id" \
    -F "contentType=MOVIE" \
    -F "video=@${video_local};type=video/mp4") || {
    err "Encoder job request failed. Check API logs."
    exit 1
  }

  job_id=$(echo "$response" | jq -r '.data.id // .data.jobId // empty' 2>/dev/null || true)
  if [ -n "$job_id" ]; then
    log "Encoder job created: id=$job_id"
    log "Poll status at: GET $API_URL/v1/a/encoder/$job_id"
  else
    log "Encoder job queued. Response: $response"
  fi
else
  log "SKIP_ENCODER=1 — skipping encoder job creation."
fi

log "──── Seed completed successfully. ──────────────────────────────────────────"
