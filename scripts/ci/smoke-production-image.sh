#!/usr/bin/env bash
# Starts /app/api and /app/import-worker from a built production image as the
# image's own user, with fly.toml's [env] and throwaway secrets, against a
# migrated Postgres on localhost. Fails unless the API answers /livez and the
# worker logs that it started, and prints both logs when either exits first.
# SMOKE_DATABASE names the database, quizzivy by default.
# Usage: smoke-production-image.sh IMAGE
set -euo pipefail

image="${1:?usage: smoke-production-image.sh IMAGE}"
export SMOKE_DATABASE="${SMOKE_DATABASE:-quizzivy}"
root="$(cd "$(dirname "$0")/../.." && pwd)"
work="$(mktemp -d)"
env_file="$work/production.env"
containers=(smoke-api smoke-worker)

cleanup() {
  docker rm --force "${containers[@]}" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

fail() {
  echo "::error::$1"
  for name in "${containers[@]}"; do
    echo "---- $name ----"
    docker logs "$name" 2>&1 || true
  done
  exit 1
}

python3 - "$root/fly.toml" > "$env_file" <<'PY'
import os
import sys
import tomllib

with open(sys.argv[1], "rb") as f:
    env = tomllib.load(f)["env"]

env.update({
    "DATABASE_URL": f"postgres://quizzivy_app:app@localhost:5432/{os.environ['SMOKE_DATABASE']}?sslmode=disable",
    "JWT_SIGNING_KEY": "smoke-signing-key-not-a-secret-0123456789abcdef",
    "JOIN_CODE_KEY": "c21va2Utam9pbi1jb2RlLWtleS1ub3Qtc2VjcmV0MDA=",
    "GOOGLE_CLIENT_SECRET": "smoke-google-secret",
    "S3_ENDPOINT": "http://localhost:9000",
    "S3_BUCKET": "quizzivy-media",
    "S3_ACCESS_KEY_ID": "smoke",
    "S3_SECRET_ACCESS_KEY": "smoke-secret",
    "S3_FORCE_PATH_STYLE": "true",
    "IMPORT_WORKER_WAKE_URL": "http://localhost:8091/wake",
    "IMPORT_WORKER_WAKE_ADDR": "localhost:8091",
})
for key, value in env.items():
    print(f"{key}={value}")
PY

user="$(docker image inspect --format '{{.Config.User}}' "$image")"
if [ "$user" != "nonroot:nonroot" ]; then
  fail "the image runs as '$user', not nonroot:nonroot"
fi

docker run --rm --network host \
  --env "MIGRATE_DATABASE_URL=postgres://quizzivy_migrate:migrate@localhost:5432/$SMOKE_DATABASE?sslmode=disable" \
  "$image" /app/migrate -dir /app/migrations up

docker run --detach --name smoke-api --network host --env-file "$env_file" "$image" /app/api >/dev/null
docker run --detach --name smoke-worker --network host --env-file "$env_file" "$image" /app/import-worker >/dev/null

running() {
  [ "$(docker inspect --format '{{.State.Running}}' "$1")" = true ]
}

api_ready=false
worker_ready=false
for _ in $(seq 1 60); do
  running smoke-api || fail "the API exited before it answered /livez"
  running smoke-worker || fail "the import worker exited before it started"
  if [ "$api_ready" = false ] && curl -fsS http://localhost:8080/livez >/dev/null 2>&1; then
    api_ready=true
  fi
  if [ "$worker_ready" = false ] && docker logs smoke-worker 2>&1 | grep -q '"msg":"import worker started"'; then
    worker_ready=true
  fi
  if [ "$api_ready" = true ] && [ "$worker_ready" = true ]; then
    break
  fi
  sleep 1
done

[ "$api_ready" = true ] || fail "the API did not answer /livez within 60 seconds"
[ "$worker_ready" = true ] || fail "the import worker did not log that it started within 60 seconds"

for name in "${containers[@]}"; do
  running "$name" || fail "$name exited after it was ready"
done
echo "The production image's API answered /livez and its import worker started, as $user."
