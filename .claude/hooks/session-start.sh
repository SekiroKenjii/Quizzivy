#!/bin/bash
# Session start for Claude Code cloud containers; never fails the session (docs/team/environment.md).

[ "${CLAUDE_CODE_REMOTE:-}" = "true" ] || exit 0

set -uo pipefail

REPO="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
LOG="${QUIZZIVY_SESSION_START_LOG:-/tmp/quizzivy-session-start.log}"
MINIO_IMAGE="${QUIZZIVY_MINIO_IMAGE:-quizzivy-minio:development}"
GOOSE_VERSION=v3.27.3
GO_TOOLCHAIN=go1.27.0
LOCAL_BIN=/root/.local/bin
MINIO_COMMIT=07c3a429bfed433e49018cb0f78a52145d4bedeb
MC_COMMIT=7394ce0dd2a80935aded936b09fa12cbb3cb8096

export_for_session() {
  [ -n "${CLAUDE_ENV_FILE:-}" ] || return 0
  local line
  for line in \
    "export GOTOOLCHAIN=$GO_TOOLCHAIN" \
    "case \":\$PATH:\" in *:$LOCAL_BIN:*) ;; *) export PATH=\"$LOCAL_BIN:\$PATH\" ;; esac"; do
    grep -qxF -- "$line" "$CLAUDE_ENV_FILE" 2>/dev/null || echo "$line" >>"$CLAUDE_ENV_FILE"
  done
}

export_for_session
echo '{"async": true, "asyncTimeout": 1500000}'

exec >>"$LOG" 2>&1
exec 9>/tmp/quizzivy-session-start.lock
if ! flock -n 9; then
  echo "$(date -u +%FT%TZ) another run holds the lock; leaving it to finish"
  exit 0
fi

export GOTOOLCHAIN="$GO_TOOLCHAIN"
export PATH="$LOCAL_BIN:$PATH"
cd "$REPO" || exit 0

step() {
  local name="$1"
  shift
  local started=$SECONDS
  echo "$(date -u +%FT%TZ) begin $name"
  if "$@"; then
    echo "$(date -u +%FT%TZ) ok    $name ($((SECONDS - started))s)"
  else
    echo "$(date -u +%FT%TZ) FAILED $name (rc $?, $((SECONDS - started))s)"
    return 1
  fi
}

start_docker() {
  if docker info >/dev/null 2>&1; then
    return 0
  fi
  setsid nohup dockerd >/tmp/dockerd.log 2>&1 &
  local i
  for i in $(seq 1 90); do
    docker info >/dev/null 2>&1 && return 0
    sleep 1
  done
  tail -n 20 /tmp/dockerd.log
  return 1
}

create_env() {
  [ -f .env ] && return 0
  cp .env.example .env || return 1
  chmod 600 .env
  local key
  key=$(openssl rand -base64 32) || return 1
  sed -i "s|^JOIN_CODE_KEY=.*|JOIN_CODE_KEY=$key|" .env
}

install_goose() {
  if [ -x "$LOCAL_BIN/goose" ] && "$LOCAL_BIN/goose" --version 2>&1 | grep -q "$GOOSE_VERSION"; then
    return 0
  fi
  mkdir -p "$LOCAL_BIN"
  GOBIN="$LOCAL_BIN" go install "github.com/pressly/goose/v3/cmd/goose@$GOOSE_VERSION"
}

install_web_dependencies() {
  [ -f web/node_modules/.modules.yaml ] && return 0
  (cd web && pnpm install --frozen-lockfile)
}

build_minio_image() {
  local work
  work=$(mktemp -d) || return 1
  (
    cd "$work" || exit 1
    fetch() {
      git init -q "$1" &&
        git -C "$1" fetch -q --depth=1 "https://github.com/minio/$1.git" "$2" &&
        git -C "$1" checkout -q --detach FETCH_HEAD
    }
    compile() {
      (cd "$1" && CGO_ENABLED=0 go build -trimpath -ldflags="$(go run buildscripts/gen-ldflags.go)" -o "../out/$1" .)
    }
    fetch minio "$MINIO_COMMIT" && fetch mc "$MC_COMMIT" || exit 1
    mkdir out licenses-minio licenses-mc
    compile minio && compile mc || exit 1
    cp minio/LICENSE minio/CREDITS licenses-minio/ && cp mc/LICENSE mc/CREDITS licenses-mc/ || exit 1
    cat >Dockerfile <<'EOF'
FROM golang:1.27-alpine AS certs
FROM alpine:3.22
LABEL org.opencontainers.image.source="https://github.com/SekiroKenjii/Quizzivy" \
      org.opencontainers.image.licenses="AGPL-3.0-or-later"
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY out/minio out/mc /usr/bin/
COPY licenses-minio/ /licenses/minio/
COPY licenses-mc/ /licenses/mc/
ENTRYPOINT ["minio"]
EOF
    docker build -q -t "$MINIO_IMAGE" .
  )
  local rc=$?
  rm -rf "$work"
  return $rc
}

ensure_minio_image() {
  docker image inspect "$MINIO_IMAGE" >/dev/null 2>&1 && return 0
  build_minio_image
}

start_postgres() {
  docker compose up -d --wait db
}

start_minio() {
  docker compose up -d --wait minio && docker compose run --rm minio-init
}

migrate() {
  local attempt
  for attempt in $(seq 1 30); do
    make migrate && return 0
    sleep 2
  done
  return 1
}

echo "$(date -u +%FT%TZ) session start hook begins in $REPO"
step "goose $GOOSE_VERSION" install_goose
step ".env" create_env
step "web dependencies" install_web_dependencies
if step "docker daemon" start_docker; then
  step "MinIO image $MINIO_IMAGE" ensure_minio_image
  step "postgres" start_postgres && step "migrations" migrate
  docker image inspect "$MINIO_IMAGE" >/dev/null 2>&1 && step "MinIO" start_minio
fi
echo "$(date -u +%FT%TZ) session start hook ends"
exit 0
