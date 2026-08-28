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
# Connection comes from standard libpq environment variables (PGHOST, PGPORT,
# PGUSER, PGPASSWORD), which is what makes it work over both a unix socket and
# TCP without a second code path.
#
# Idempotent: safe to re-run.
set -euo pipefail

MIGRATE_PASSWORD="${QUIZZIVY_MIGRATE_PASSWORD:-migrate}"
APP_PASSWORD="${QUIZZIVY_APP_PASSWORD:-app}"
DB_NAME="${QUIZZIVY_DB_NAME:-quizzivy}"

psql -v ON_ERROR_STOP=1 --dbname postgres <<SQL
DO \$\$
BEGIN
  -- Owns the schema and runs goose. Needs CREATE on the database so it can
  -- install the trusted extensions pg_trgm and unaccent.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_migrate') THEN
    CREATE ROLE quizzivy_migrate LOGIN PASSWORD '${MIGRATE_PASSWORD}';
  END IF;

  -- What the API connects as. Owns nothing, cannot run DDL.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_app') THEN
    CREATE ROLE quizzivy_app LOGIN PASSWORD '${APP_PASSWORD}';
  END IF;
END
\$\$;
SQL

if ! psql -tAc "SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}'" --dbname postgres | grep -q 1; then
  psql -v ON_ERROR_STOP=1 --dbname postgres \
    -c "CREATE DATABASE ${DB_NAME} OWNER quizzivy_migrate"
fi

psql -v ON_ERROR_STOP=1 --dbname postgres \
  -c "GRANT CONNECT ON DATABASE ${DB_NAME} TO quizzivy_app"

# Deny the app role anything in public; everything it needs is in schema app,
# granted by migration 00022.
psql -v ON_ERROR_STOP=1 --dbname "${DB_NAME}" <<'SQL'
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO quizzivy_migrate, quizzivy_app;
SQL

echo "quizzivy: roles and database ready"
