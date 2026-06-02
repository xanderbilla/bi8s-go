#!/usr/bin/env bash
# seed.sh — Wipe content/persons, seed via API.
#
# Persons: Angelina Jolie (unverified), Robert Downey Jr. (verified=true)
# Movies:  Eternals (MOVIE), Marvel Anime: Iron Man (TV)
#
# SKIP_WIPE=1  — skip wipe phase
# SKIP_SEED=1  — skip seed phase
set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$( cd "$SCRIPT_DIR/.." && pwd )"

API_URL="${API_URL:-http://localhost:8080}"
AWS_REGION="${AWS_REGION:-us-east-1}"
SKIP_WIPE="${SKIP_WIPE:-0}"
SKIP_SEED="${SKIP_SEED:-0}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-60}"
RETRY_INTERVAL="${RETRY_INTERVAL:-10}"

log() { echo "[seed] $(date '+%H:%M:%S') $*"; }
err() { echo "[seed] $(date '+%H:%M:%S') ERROR: $*" >&2; }

IMG_DIR="$REPO_ROOT/assets/images"
VID_DIR="$REPO_ROOT/assets/videos"
_project="${PROJECT_NAME:-bi8s}"
_env="${APP_ENV:-dev}"

for _cmd in aws jq curl; do
  command -v "$_cmd" > /dev/null 2>&1 || { err "$_cmd not found."; exit 1; }
done

_wipe_dynamo_table() {
  local table_name="$1"
  local total_deleted=0
  local start_key=""

  local key_attrs
  key_attrs=$(aws dynamodb describe-table \
    --table-name "$table_name" \
    --region "$AWS_REGION" \
    --query 'Table.KeySchema[].AttributeName' \
    --output json)

  while :; do
    local scan_args=(
      --table-name "$table_name"
      --region "$AWS_REGION"
      --output json
    )
    if [ -n "$start_key" ]; then
      scan_args+=(--exclusive-start-key "$start_key")
    fi

    local scan_result
    scan_result=$(aws dynamodb scan "${scan_args[@]}")

    while IFS= read -r key_json; do
      [ -z "$key_json" ] && continue
      aws dynamodb delete-item \
        --table-name "$table_name" \
        --key "$key_json" \
        --region "$AWS_REGION" \
        --output json > /dev/null
      total_deleted=$(( total_deleted + 1 ))
    done < <(echo "$scan_result" | jq -c --argjson keys "$key_attrs" '.Items[] | . as $it | (reduce $keys[] as $k ({}; . + {($k): $it[$k]}))')

    local next_key
    next_key=$(echo "$scan_result" | jq -rc '.LastEvaluatedKey // ""')
    if [ -z "$next_key" ] || [ "$next_key" = "null" ]; then
      break
    fi
    start_key="$next_key"
  done

  log "wiped $table_name ($total_deleted items)"
}

if [ "$SKIP_WIPE" = "0" ]; then
  log "Phase 1: wipe"
  _wipe_dynamo_table "${DYNAMODB_ATTRIBUTE_TABLE:-${_project}-attributes-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_CONTENT_TABLE:-${_project}-content-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_PERSON_TABLE:-${_project}-person-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_CONTENT_CAST_TABLE:-${_project}-content-cast-table-${_env}}"
  _wipe_dynamo_table "${DYNAMODB_CONTENT_ATTRIBUTE_TABLE:-${_project}-content-attribute-table-${_env}}"
  log "wipe done"
else
  log "SKIP_WIPE=1, skipping wipe"
fi

log "Phase 2: health check"
attempt=0
until curl -sf --max-time 5 "$API_URL/v1/health" > /dev/null 2>&1; do
  attempt=$(( attempt + 1 ))
  [ "$attempt" -ge "$MAX_ATTEMPTS" ] && { err "API not healthy after $(( MAX_ATTEMPTS * RETRY_INTERVAL ))s"; exit 1; }
  log "  attempt $attempt/$MAX_ATTEMPTS, retrying in ${RETRY_INTERVAL}s..."
  sleep "$RETRY_INTERVAL"
done
log "API healthy"

_api_post() {
  local endpoint="$1"; shift
  curl -sf --max-time 60 -X POST "${API_URL}${endpoint}" "$@"
}

if [ "$SKIP_SEED" = "0" ]; then
  log "Phase 3: seed"
  declare -A _attr_id
  declare -A _person_id

  _create_attribute() {
    local key="$1" name="$2" attr_type="$3"
    local resp id http_code
    http_code=$(curl -s -o /tmp/_attr_resp.json -w "%{http_code}" --max-time 60 -X POST "${API_URL}/v1/a/attributes/" \
      -F "name=${name}" \
      -F "attribute_type=${attr_type}")
    resp=$(cat /tmp/_attr_resp.json)
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
      id=$(echo "$resp" | jq -r '.data.id')
    elif [ "$http_code" = "409" ]; then
      id=$(curl -sf --max-time 30 "${API_URL}/v1/a/attributes/" \
        | jq -r --arg n "$name" '.data[] | select(.name == $n) | .id')
      [ -z "$id" ] && { err "attribute $name exists but lookup failed"; exit 1; }
      log "attribute $name already exists (id=$id)"
    else
      err "failed to create attribute $name (http $http_code): $resp"; exit 1
    fi
    _attr_id["$key"]="$id"
  }

  _create_attribute "action"         "Action"         "GENRE"
  _create_attribute "drama"          "Drama"          "GENRE"
  _create_attribute "sci-fi"         "Sci-Fi"         "GENRE"
  _create_attribute "anime"          "Anime"          "GENRE"
  _create_attribute "fantasy"        "Fantasy"        "GENRE"
  _create_attribute "epic"           "Epic"           "MOOD,TAG"
  _create_attribute "marvel-studios" "Marvel Studios" "STUDIO"
  _create_attribute "a1-pictures"    "A-1 Pictures"   "STUDIO"
  _create_attribute "superhero"      "Superhero"      "CATEGORY,SPECIALITY"

  declare -A _content_id

  _create_person() {
    local key="$1" name="$2"; shift 2
    local resp id http_code
    http_code=$(curl -s -o /tmp/_person_resp.json -w "%{http_code}" --max-time 60 -X POST "${API_URL}/v1/a/people/" \
      -F "name=${name}" "$@")
    resp=$(cat /tmp/_person_resp.json)
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
      id=$(echo "$resp" | jq -r '.data.id')
    elif [ "$http_code" = "409" ]; then
      id=$(curl -sf --max-time 30 "${API_URL}/v1/a/people/" \
        | jq -r --arg n "$name" '.data[] | select(.name == $n) | .id' | head -1)
      [ -z "$id" ] || [ "$id" = "null" ] && { err "person $name exists but lookup failed"; exit 1; }
      log "person $name already exists (id=$id)"
    else
      err "failed to create person $name (http $http_code): $resp"; exit 1
    fi
    _person_id["$key"]="$id"
    log "person: $name → $id"
  }

  _create_content() {
    local key="$1" title="$2"; shift 2
    local resp id http_code
    http_code=$(curl -s -o /tmp/_content_resp.json -w "%{http_code}" --max-time 60 -X POST "${API_URL}/v1/a/content/" \
      -F "title=${title}" "$@")
    resp=$(cat /tmp/_content_resp.json)
    if [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
      id=$(echo "$resp" | jq -r '.data.id')
    elif [ "$http_code" = "409" ]; then
      id=$(curl -sf --max-time 30 "${API_URL}/v1/a/content/" \
        | jq -r --arg t "$title" '.data[] | select(.title == $t) | .id' | head -1)
      [ -z "$id" ] || [ "$id" = "null" ] && { err "content $title exists but lookup failed"; exit 1; }
      log "content $title already exists (id=$id)"
    else
      err "failed to create content $title (http $http_code): $resp"; exit 1
    fi
    _content_id["$key"]="$id"
    log "content: $title → $id"
  }

  _create_person "648032" "Angelina Jolie" \
    -F "stage_name=AJ" \
    -F "roles=PERFORMER,CONTENT_CREATOR" \
    -F "bio=Angelina Jolie is an American actress, filmmaker, and humanitarian. Known for iconic roles in Lara Croft: Tomb Raider, Mr. & Mrs. Smith, Maleficent, and Eternals, she is one of the most celebrated and influential figures in Hollywood. She won an Academy Award for Best Supporting Actress for Girl, Interrupted." \
    -F "birth_date=1975-06-04" \
    -F "birth_place=Los Angeles, California" \
    -F "nationality=American" \
    -F "gender=Female" \
    -F "height=173" \
    -F "active=true" \
    -F "verified=false" \
    -F "debut_year=1993" \
    -F "career_status=Active" \
    -F "measurements_bust=36" \
    -F "measurements_waist=27" \
    -F "measurements_hips=36" \
    -F "measurements_unit=inches" \
    -F "measurements_body_type=Slim" \
    -F "measurements_eye_color=Green" \
    -F "measurements_hair_color=Black" \
    -F "tags=${_attr_id[epic]}:Epic" \
    -F "categories=${_attr_id[superhero]}:Superhero" \
    -F "specialties=${_attr_id[superhero]}:Superhero" \
    -F "profile=@${IMG_DIR}/persons/person-542131-profile.jpg;type=image/jpeg" \
    -F "backdrop=@${IMG_DIR}/persons/person-542131-backdrop.jpg;type=image/jpeg"

  _create_person "542131" "Robert Downey Jr." \
    -F "stage_name=RDJ" \
    -F "roles=PERFORMER,CONTENT_CREATOR" \
    -F "bio=Robert John Downey Jr. is an American actor renowned for his portrayal of Tony Stark / Iron Man in the Marvel Cinematic Universe. With a career spanning over five decades, he is celebrated as one of the most talented and versatile actors of his generation, earning two Academy Award nominations and a BAFTA." \
    -F "birth_date=1965-04-04" \
    -F "birth_place=Manhattan, New York City, NY" \
    -F "nationality=American" \
    -F "gender=Male" \
    -F "height=174" \
    -F "active=true" \
    -F "verified=true" \
    -F "debut_year=1970" \
    -F "career_status=Active" \
    -F "measurements_unit=cm" \
    -F "measurements_bust=102" \
    -F "measurements_waist=81" \
    -F "measurements_hips=99" \
    -F "measurements_body_type=Athletic" \
    -F "measurements_eye_color=Brown" \
    -F "measurements_hair_color=Brown" \
    -F "tags=${_attr_id[epic]}:Epic" \
    -F "categories=${_attr_id[superhero]}:Superhero" \
    -F "specialties=${_attr_id[superhero]}:Superhero" \
    -F "profile=@${IMG_DIR}/persons/person-648032-profile.jpg;type=image/jpeg" \
    -F "backdrop=@${IMG_DIR}/persons/person-648032-backdrop.jpg;type=image/jpeg"


  _create_content "eternals" "Eternals" \
    -F "overview=The Eternals, a race of immortal beings with superhuman powers who have secretly lived on Earth for thousands of years, reunite to battle the monstrous Deviants and uncover a startling secret about their own existence." \
    -F "content_type=MOVIE" \
    -F "status=RELEASED" \
    -F "visibility=PUBLIC" \
    -F "adult=false" \
    -F "content_rating=18_PLUS" \
    -F "original_language=en" \
    -F "runtime=156" \
    -F "release_date=2021-11-05" \
    -F "tagline=In the beginning..." \
    -F "origin_country=US" \
    -F "genres=${_attr_id[action]}:Action,${_attr_id[sci-fi]}:Sci-Fi" \
    -F "casts=${_person_id[648032]}:Angelina Jolie,${_person_id[542131]}:Robert Downey Jr." \
    -F "tags=${_attr_id[epic]}:Epic" \
    -F "mood_tags=${_attr_id[epic]}:Epic" \
    -F "studios=${_attr_id[marvel-studios]}:Marvel Studios" \
    -F "poster=@${IMG_DIR}/movies/ironman-poster.jpg;type=image/jpeg" \
    -F "cover=@${IMG_DIR}/movies/ironman-backdrop.jpg;type=image/jpeg"

  _create_content "ironman-anime" "Marvel Anime: Iron Man" \
    -F "overview=Tony Stark travels to Japan to build a new arc reactor and unveil Iron Man Dio. When the villainous ZODIAC steals the suit, Tony dons the original armor to stop them in an epic clash across the neon-lit streets of Tokyo." \
    -F "content_type=TV" \
    -F "status=ENDED" \
    -F "visibility=PUBLIC" \
    -F "adult=false" \
    -F "content_rating=18_PLUS" \
    -F "original_language=ja" \
    -F "runtime=24" \
    -F "first_air_date=2010-10-01" \
    -F "tagline=Forged in fire. Reborn in iron." \
    -F "origin_country=JP" \
    -F "genres=${_attr_id[action]}:Action,${_attr_id[drama]}:Drama" \
    -F "casts=${_person_id[542131]}:Robert Downey Jr.,${_person_id[648032]}:Angelina Jolie" \
    -F "tags=${_attr_id[epic]}:Epic" \
    -F "mood_tags=${_attr_id[epic]}:Epic" \
    -F "studios=${_attr_id[marvel-studios]}:Marvel Studios" \
    -F "poster=@${IMG_DIR}/movies/eternals-poster.jpg;type=image/jpeg" \
    -F "cover=@${IMG_DIR}/movies/eternals-backdrop.jpg;type=image/jpeg"

  _create_content "solo-leveling" "Solo Leveling" \
    -F "overview=After a brutal battle that nearly kills him, the weakest hunter Sung Jin-Woo gains an unprecedented ability to level up in strength, launching his rise from the bottom to the most powerful hunter in the world." \
    -F "content_type=TV" \
    -F "status=RETURNING_SERIES" \
    -F "visibility=PUBLIC" \
    -F "adult=false" \
    -F "content_rating=18_PLUS" \
    -F "original_language=ja" \
    -F "runtime=24" \
    -F "first_air_date=2024-01-06" \
    -F "tagline=Arise." \
    -F "origin_country=JP,KR" \
    -F "genres=${_attr_id[anime]}:Anime,${_attr_id[action]}:Action,${_attr_id[fantasy]}:Fantasy" \
    -F "tags=${_attr_id[epic]}:Epic" \
    -F "mood_tags=${_attr_id[epic]}:Epic" \
    -F "studios=${_attr_id[a1-pictures]}:A-1 Pictures" \
    -F "poster=@${IMG_DIR}/movies/solo-leveling-poster.jpg;type=image/jpeg" \
    -F "cover=@${IMG_DIR}/movies/solo-leveling-backdrop.jpg;type=image/jpeg"


  log "seed done"
else
  log "SKIP_SEED=1, skipping seed"
fi

log "done"

