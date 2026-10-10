#!/usr/bin/env bash
# Usage: scripts/dev/live-stack.sh up|down|status <name>
# A private stack for a live check: database qa_<name> (migrated and seeded), the API
# and `vite preview` on ports derived from <name>, state under /tmp/quizzivy-live/<name>/.
# It needs Postgres and MinIO already running (docker compose up -d --wait db minio) and
# never takes the heavy lock. `down` stops both servers, drops the database and removes
# the state directory; objects the API stored in the compose buckets stay.
set -euo pipefail

readonly ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
readonly LIVE_DIR=/tmp/quizzivy-live
readonly PG_HOST=${LIVE_PG_HOST:-localhost}
readonly PG_PORT=${LIVE_PG_PORT:-5432}
readonly PG_ADMIN_URL=${LIVE_PG_ADMIN_URL:-postgres://postgres:${POSTGRES_SUPERUSER_PASSWORD:-postgres}@${PG_HOST}:${PG_PORT}/postgres}
readonly MIGRATE_PASSWORD=${QUIZZIVY_MIGRATE_PASSWORD:-migrate}
readonly APP_PASSWORD=${QUIZZIVY_APP_PASSWORD:-app}

fail() {
  echo "live-stack: $*" >&2
  exit 1
}

port_free() {
  ! (echo >"/dev/tcp/127.0.0.1/$1") 2>/dev/null
}

derive_ports() {
  local offset candidate
  offset=$(($(printf '%s' "$NAME" | cksum | cut -d' ' -f1) % 500))
  for candidate in $(seq 0 19); do
    API_PORT=$((20000 + (offset + candidate) % 500))
    WEB_PORT=$((20500 + (offset + candidate) % 500))
    if port_free "$API_PORT" && port_free "$WEB_PORT"; then
      return
    fi
  done
  fail "no free port pair near $((20000 + offset))"
}

alive() {
  local pid_file=$1
  [ -s "$pid_file" ] && kill -0 "$(cat "$pid_file")" 2>/dev/null
}

start_detached() {
  local pid_file=$1 log_file=$2
  shift 2
  setsid bash -c 'echo $$ >"$0"; exec "$@"' "$pid_file" "$@" >"$log_file" 2>&1 </dev/null &
  local waited=0
  while [ ! -s "$pid_file" ] && [ "$waited" -lt 50 ]; do
    sleep 0.1
    waited=$((waited + 1))
  done
}

stop_detached() {
  local pid_file=$1 pid waited=0
  [ -s "$pid_file" ] || return 0
  pid=$(cat "$pid_file")
  kill -TERM -- "-$pid" 2>/dev/null || true
  while kill -0 "$pid" 2>/dev/null && [ "$waited" -lt 50 ]; do
    sleep 0.1
    waited=$((waited + 1))
  done
  kill -KILL -- "-$pid" 2>/dev/null || true
  rm -f "$pid_file"
}

wait_for() {
  local url=$1 label=$2 waited=0
  until curl -fsS -o /dev/null "$url" 2>/dev/null; do
    waited=$((waited + 1))
    [ "$waited" -le 60 ] || fail "$label did not answer at $url within 60 s; see $DIR/*.log"
    sleep 1
  done
}

database_exists() {
  [ "$(psql -X -At "$PG_ADMIN_URL" -c "select 1 from pg_database where datname = '$DB'")" = 1 ]
}

write_env() {
  {
    echo "API_PORT=$API_PORT"
    echo "WEB_PORT=$WEB_PORT"
    echo "JWT_SIGNING_KEY=$(openssl rand -base64 48 | tr -d '\n')"
    echo "JOIN_CODE_KEY=$(openssl rand -base64 32 | tr -d '\n')"
  } >"$DIR/env"
  chmod 600 "$DIR/env"
}

up() {
  if alive "$DIR/api.pid" || alive "$DIR/web.pid"; then
    status
    return
  fi
  psql -X -At "$PG_ADMIN_URL" -c 'select 1' >/dev/null 2>&1 || fail "Postgres is not reachable at $PG_HOST:$PG_PORT (docker compose up -d --wait db minio)"
  curl -fsS -o /dev/null "http://localhost:9000/minio/health/live" || fail "MinIO is not reachable on :9000 (docker compose up -d --wait minio, then docker compose run --rm minio-init)"

  mkdir -p "$DIR"
  [ -s "$DIR/env" ] || { derive_ports; write_env; }
  set -a
  # shellcheck disable=SC1091
  . "$DIR/env"
  set +a

  local migrate_url="postgres://quizzivy_migrate:${MIGRATE_PASSWORD}@${PG_HOST}:${PG_PORT}/${DB}?sslmode=disable"
  if ! database_exists; then
    psql -X -v ON_ERROR_STOP=1 "$PG_ADMIN_URL" -c "create database \"$DB\" owner quizzivy_migrate" >/dev/null
    goose -dir "$ROOT/migrations" postgres "$migrate_url" up >"$DIR/migrate.log" 2>&1 || fail "migrate failed; see $DIR/migrate.log"
    local seed
    for seed in "$ROOT"/seed/*.sql; do
      psql -X -v ON_ERROR_STOP=1 "$migrate_url" -f "$seed" >>"$DIR/seed.log" 2>&1 || fail "seed $seed failed; see $DIR/seed.log"
    done
  fi

  (cd "$ROOT/server" && go build -o "$DIR/api" ./cmd/api) || fail "go build failed (GOTOOLCHAIN=go1.27.0 may be needed)"
  (cd "$ROOT/web" && VITE_API_BASE_URL="http://localhost:$API_PORT" VITE_RICH_OPTION_EDITOR=true pnpm exec vite build --outDir "$DIR/dist" --emptyOutDir >"$DIR/build.log" 2>&1) || fail "vite build failed; see $DIR/build.log"

  start_detached "$DIR/api.pid" "$DIR/api.log" env \
    DATABASE_URL="postgres://quizzivy_app:${APP_PASSWORD}@${PG_HOST}:${PG_PORT}/${DB}?sslmode=disable" \
    API_PORT="$API_PORT" \
    CORS_ALLOWED_ORIGINS="http://localhost:$WEB_PORT" \
    JWT_SIGNING_KEY="$JWT_SIGNING_KEY" JOIN_CODE_KEY="$JOIN_CODE_KEY" \
    REFRESH_COOKIE_SECURE=false DOCS_PUBLIC=true \
    S3_ENDPOINT=http://localhost:9000 S3_REGION=auto S3_BUCKET=quizzivy-media \
    S3_ACCESS_KEY_ID=quizzivy S3_SECRET_ACCESS_KEY=quizzivy-dev-secret S3_FORCE_PATH_STYLE=true \
    "$DIR/api"
  wait_for "http://localhost:$API_PORT/livez" "the API"

  start_detached "$DIR/web.pid" "$DIR/web.log" \
    "$ROOT/web/node_modules/.bin/vite" preview "$ROOT/web" --outDir "$DIR/dist" --port "$WEB_PORT" --strictPort
  wait_for "http://localhost:$WEB_PORT/" "vite preview"
  status
}

down() {
  stop_detached "$DIR/web.pid"
  stop_detached "$DIR/api.pid"
  if database_exists; then
    psql -X -v ON_ERROR_STOP=1 "$PG_ADMIN_URL" -c "drop database \"$DB\" with (force)" >/dev/null
  fi
  rm -rf "$DIR"
  echo "stack $NAME: down"
}

status() {
  if [ ! -s "$DIR/env" ]; then
    echo "stack $NAME: not created"
    return
  fi
  set -a
  # shellcheck disable=SC1091
  . "$DIR/env"
  set +a
  echo "stack $NAME"
  echo "  database  $DB $(database_exists && echo present || echo missing)"
  echo "  api       http://localhost:$API_PORT $(alive "$DIR/api.pid" && echo "pid $(cat "$DIR/api.pid")" || echo stopped)"
  echo "  web       http://localhost:$WEB_PORT $(alive "$DIR/web.pid" && echo "pid $(cat "$DIR/web.pid")" || echo stopped)"
  echo "  state     $DIR"
}

[ "$#" -eq 2 ] || fail "usage: live-stack.sh up|down|status <name>"
readonly ACTION=$1 NAME=$2
[[ "$NAME" =~ ^[a-z0-9][a-z0-9_]{0,24}$ ]] || fail "name must match [a-z0-9][a-z0-9_]{0,24}"
readonly DB="qa_$NAME"
readonly DIR="$LIVE_DIR/$NAME"

case "$ACTION" in
  up) up ;;
  down) down ;;
  status) status ;;
  *) fail "unknown action $ACTION (up, down or status)" ;;
esac
