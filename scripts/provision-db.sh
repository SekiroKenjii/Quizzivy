#!/usr/bin/env bash
# Creates the two database roles and the quizzivy database.
#
# Roles live here rather than in a migration because CREATE ROLE needs superuser
# and quizzivy_migrate deliberately is not one (spec §13.5).
#
# ONE script, used by three callers, so they cannot drift:
#   - docker/initdb/  on first container start, locally
#   - CI              against the postgres service container
#   - production      run once by hand, per docs/plan/20-data-model.md §1
#
# Connection is either a DSN as the first argument, or standard libpq
# environment variables (PGHOST, PGPORT, PGUSER, PGPASSWORD). The DSN form is
# what a managed provider hands you; the env form is what the docker initdb hook
# already has. Supporting both avoids a second copy of this logic.
#
#   ./scripts/provision-db.sh                              # libpq env vars
#   ./scripts/provision-db.sh "postgres://user:pw@host/db" # DSN (Neon, etc.)
#
# Idempotent: safe to re-run.
set -euo pipefail

ADMIN_DSN="${1:-}"

# psql() wrapper: with a DSN, connect to the named database ON that host; with
# env vars, let libpq resolve the connection.
run_psql() {
  local dbname="$1"; shift
  if [ -n "$ADMIN_DSN" ]; then
    # Swap the database component of the DSN, keeping host, credentials and
    # any query string (Neon requires sslmode=require).
    local rewritten
    rewritten=$(printf '%s' "$ADMIN_DSN" | sed -E "s#(://[^/]+)/[^?]*#\1/${dbname}#")
    psql -v ON_ERROR_STOP=1 "$rewritten" "$@"
  else
    psql -v ON_ERROR_STOP=1 --dbname "$dbname" "$@"
  fi
}

MIGRATE_PASSWORD="${QUIZZIVY_MIGRATE_PASSWORD:-migrate}"
APP_PASSWORD="${QUIZZIVY_APP_PASSWORD:-app}"
DB_NAME="${QUIZZIVY_DB_NAME:-quizzivy}"

run_psql postgres <<SQL
DO \$\$
BEGIN
  -- Owns the schema and runs goose. Needs CREATE on the database so it can
  -- install the trusted extensions pg_trgm and unaccent.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_migrate') THEN
    CREATE ROLE quizzivy_migrate LOGIN;
  END IF;

  -- What the API connects as. Owns nothing, cannot run DDL.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_app') THEN
    CREATE ROLE quizzivy_app LOGIN;
  END IF;
END
\$\$;

-- Set the password unconditionally rather than only on creation, so a re-run
-- with different credentials converges instead of silently keeping the old
-- ones and handing back a DSN that cannot authenticate.
ALTER ROLE quizzivy_migrate PASSWORD '${MIGRATE_PASSWORD}';
ALTER ROLE quizzivy_app     PASSWORD '${APP_PASSWORD}';

-- CREATEDB is for the test suite, not for production.
--
-- TestMigrationsAreReversible drops the whole schema to prove every Down works.
-- It used to do that in the shared test database, which "go test ./..." runs
-- packages against IN PARALLEL -- so it deleted app.users out from under
-- internal/classes mid-query and turned develop CI red for 12 straight runs.
-- The test now creates a scratch database, does its damage there, and drops it,
-- which needs this. On Neon the migrate role is not provisioned by this script
-- and does not get it.
--
-- NOTE FOR EDITORS: this heredoc is UNQUOTED, so that the password variables
-- expand. That also makes backticks command substitution and makes a dollar
-- sign followed by a brace a parameter expansion -- in SQL COMMENTS too, which
-- bash never sees as comments. An earlier version of this note quoted a shell
-- command in backticks and provisioning ran the test suite and piped its output
-- into psql; the version after that used a dollar-brace and got "bad
-- substitution". Keep both characters out of this block.
ALTER ROLE quizzivy_migrate CREATEDB;
SQL

# Membership BEFORE the database is created, not after.
#
# CREATE DATABASE ... OWNER <role> requires the creator to be able to SET ROLE
# to that owner. A superuser always can, so this is invisible locally; on a
# managed provider the admin is only CREATEROLE, and the ordering bug surfaces
# as "must be able to SET ROLE quizzivy_migrate" -- after the roles have already
# been created, leaving a half-provisioned database.
run_psql postgres -c "GRANT quizzivy_migrate TO CURRENT_USER WITH SET TRUE" 2>/dev/null \
  || run_psql postgres -c "GRANT quizzivy_migrate TO CURRENT_USER" 2>/dev/null \
  || true

if ! run_psql postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" | grep -q 1; then
  run_psql postgres -c "CREATE DATABASE ${DB_NAME} OWNER quizzivy_migrate"
fi

run_psql postgres -c "GRANT CONNECT ON DATABASE ${DB_NAME} TO quizzivy_app"


# Deny the app role anything in public; everything it needs is in schema app,
# granted by migration 00022.
run_psql "${DB_NAME}" <<'SQL'
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO quizzivy_migrate, quizzivy_app;
SQL

echo "quizzivy: roles and database ready"
