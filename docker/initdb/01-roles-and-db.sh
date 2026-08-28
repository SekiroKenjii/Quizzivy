#!/bin/bash
# Runs once, as superuser, on first container start.
#
# Roles live here rather than in a migration because CREATE ROLE requires
# superuser and the migration role is deliberately not one (spec §13.5).
# Production has the same steps as a runbook -- see docs/plan/20-data-model.md §1.
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres <<-SQL
    -- Owns the schema, runs goose. Has CREATE on the database so it can
    -- install the trusted extensions pg_trgm and unaccent.
    CREATE ROLE quizzivy_migrate LOGIN PASSWORD '${QUIZZIVY_MIGRATE_PASSWORD}';

    -- What the API connects as. Owns nothing, cannot run DDL.
    CREATE ROLE quizzivy_app LOGIN PASSWORD '${QUIZZIVY_APP_PASSWORD}';

    CREATE DATABASE quizzivy OWNER quizzivy_migrate;
    GRANT CONNECT ON DATABASE quizzivy TO quizzivy_app;
SQL

# Deny the app role anything in public; everything it needs is in schema app,
# granted by migration 00022.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname quizzivy <<-SQL
    REVOKE ALL ON SCHEMA public FROM PUBLIC;
    GRANT USAGE ON SCHEMA public TO quizzivy_migrate, quizzivy_app;
SQL

echo "quizzivy: roles and database created"
