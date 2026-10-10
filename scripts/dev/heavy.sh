#!/usr/bin/env bash
# Usage: scripts/dev/heavy.sh <command> [args...]
#        scripts/dev/heavy.sh --status
# Runs a heavy command (a full suite, vite build, Playwright, the Go integration and
# e2e tiers) in one of HEAVY_SLOTS machine-wide slots, two by default, and waits for
# whichever frees first. It also holds /tmp/quizzivy-heavy.lock in shared mode, so a
# caller still using `flock -o /tmp/quizzivy-heavy.lock` excludes it and is excluded
# by it. The command runs without the lock descriptors, so a server it starts never
# holds a slot. Never start a server under it.
set -u

readonly LOCK_DIR=${HEAVY_LOCK_DIR:-/tmp}
readonly LEGACY_LOCK=$LOCK_DIR/quizzivy-heavy.lock
readonly SLOT_COUNT=${HEAVY_SLOTS:-2}

slot_file() {
  printf '%s/quizzivy-heavy.%s.lock' "$LOCK_DIR" "$1"
}

print_status() {
  local n file
  for n in $(seq 1 "$SLOT_COUNT"); do
    file=$(slot_file "$n")
    if ( flock -n 9 ) 9>>"$file" 2>/dev/null; then
      printf 'slot %s: free\n' "$n"
    else
      printf 'slot %s: %s\n' "$n" "$(cat "$file" 2>/dev/null)"
    fi
  done
  if ( flock -n -s 9 ) 9>>"$LEGACY_LOCK" 2>/dev/null; then
    printf 'legacy lock: not held exclusively\n'
  else
    printf 'legacy lock: held exclusively by a flock -o caller\n'
  fi
}

take_slot() {
  local n file
  while true; do
    for n in $(seq 1 "$SLOT_COUNT"); do
      file=$(slot_file "$n")
      exec 8>>"$file"
      if flock -n 8; then
        SLOT=$n
        SLOT_FILE=$file
        return
      fi
      exec 8>&-
    done
    sleep 1
  done
}

if [ "$#" -eq 0 ]; then
  echo "usage: heavy.sh <command> [args...] | --status" >&2
  exit 2
fi

if [ "$1" = "--status" ]; then
  print_status
  exit 0
fi

waited_from=$(date +%s)
take_slot
exec 9>>"$LEGACY_LOCK"
flock -s 9

printf 'pid=%s since=%s cmd=%s\n' "$$" "$(date -u +%H:%M:%SZ)" "$*" >"$SLOT_FILE"
echo "heavy: slot $SLOT after $(($(date +%s) - waited_from)) s" >&2

"$@" <&0 8>&- 9>&- &
child=$!
trap 'kill -TERM "$child" 2>/dev/null' INT TERM HUP
wait "$child"
status=$?
while kill -0 "$child" 2>/dev/null; do
  wait "$child"
  status=$?
done
: >"$SLOT_FILE"
exit "$status"
