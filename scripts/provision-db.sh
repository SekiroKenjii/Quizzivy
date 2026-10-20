#!/usr/bin/env bash
# Creates the three database roles and the quizzivy database.
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
# Idempotent: safe to re-run. A re-run changes a role's password only when the
# matching QUIZZIVY_*_PASSWORD variable is set, so adding one role to a database
# that is already in use never touches the credentials of the others. Against a
# database that is not local it also refuses to create a role without that
# variable, never changes the password of an existing migrate or app role, and
# never grants CREATEDB. The last lines it prints say which roles it set a
# password for.
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

MIGRATE_PASSWORD="${QUIZZIVY_MIGRATE_PASSWORD:-}"
APP_PASSWORD="${QUIZZIVY_APP_PASSWORD:-}"
MAINTENANCE_PASSWORD="${QUIZZIVY_MAINTENANCE_PASSWORD:-}"
DB_NAME="${QUIZZIVY_DB_NAME:-quizzivy}"

# A local run is the docker initdb hook, CI and a developer's own Postgres. Only
# there may the script invent a password (the development defaults below) or
# grant CREATEDB. Anything else is a managed database: it must be told every
# password it is to set, and it never gets CREATEDB.
connection_host() {
  if [ -n "$ADMIN_DSN" ]; then
    printf '%s' "$ADMIN_DSN" | sed -E 's#^[A-Za-z][A-Za-z0-9+.-]*://([^@/]*@)?(\[[^]]*\]|[^:/?]*).*$#\2#'
  else
    printf '%s' "${PGHOST:-}"
  fi
}

# A tunnel to a production database (fly proxy, an SSH forward) arrives on
# localhost and looks local; QUIZZIVY_MANAGED=1 says it is not.
CONNECTION_HOST=$(connection_host)
case "$CONNECTION_HOST" in
  ""|/*) LOCAL=yes ;;
  localhost|127.0.0.1|::1|"[::1]")
    LOCAL=yes
    if [ "${QUIZZIVY_MANAGED:-}" != 1 ]; then
      echo "provision-db: $CONNECTION_HOST is treated as a local database; if it is a tunnel to a production database, stop and run this with QUIZZIVY_MANAGED=1" >&2
    fi
    ;;
  *) LOCAL=no ;;
esac
if [ "${QUIZZIVY_MANAGED:-}" = 1 ]; then
  LOCAL=no
fi

EXISTING_ROLES=$(run_psql postgres -tAc "SELECT rolname FROM pg_roles WHERE rolname IN ('quizzivy_migrate', 'quizzivy_app', 'quizzivy_maintenance')")

role_exists() {
  grep -qx "$1" <<<"$EXISTING_ROLES"
}

# Prints the password to set for a role, or nothing when the role is to keep
# the one it has. The credentials of a running system are not this script's to
# overwrite, and a re-run to add one role must not touch the others, so:
#   - a role that exists keeps its password unless a variable names a new one;
#   - on a managed database even that is refused for the migrate and app roles,
#     whose passwords are rotated in the provider's console, because a shell
#     that sourced .env carries the development values in these variables;
#   - on a managed database a role that does not exist needs its variable, and a
#     development default is never accepted there.
password_to_set() {
  local role="$1" variable="$2" supplied="$3" fallback="$4"
  if role_exists "$role"; then
    if [ -z "$supplied" ]; then
      return 0
    fi
    if [ "$LOCAL" = no ] && [ "$role" != quizzivy_maintenance ]; then
      echo "provision-db: $role exists, so $variable is ignored; rotate its password in the provider's console" >&2
      return 0
    fi
  elif [ -z "$supplied" ]; then
    if [ "$LOCAL" = yes ]; then
      printf '%s' "$fallback"
      return 0
    fi
    echo "provision-db: $role does not exist and $variable is not set; a managed database gets no default password" >&2
    return 1
  fi
  if [ "$LOCAL" = no ] && [ "$supplied" = "$fallback" ]; then
    echo "provision-db: $variable is the development default; a managed database needs its own password" >&2
    return 1
  fi
  printf '%s' "$supplied"
}

migrate_pw=$(password_to_set quizzivy_migrate QUIZZIVY_MIGRATE_PASSWORD "$MIGRATE_PASSWORD" migrate)
app_pw=$(password_to_set quizzivy_app QUIZZIVY_APP_PASSWORD "$APP_PASSWORD" app)
maintenance_pw=$(password_to_set quizzivy_maintenance QUIZZIVY_MAINTENANCE_PASSWORD "$MAINTENANCE_PASSWORD" maintenance)

run_psql postgres <<'SQL'
DO $$
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

  -- What cmd/maintenance connects as, from the operator's machine and from the
  -- nightly retention workflow (T-R5.17a, D16). Owns nothing, creates nothing,
  -- and holds only the grants the migration that names it gives it. Its
  -- credential is never set on the API.
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_maintenance') THEN
    CREATE ROLE quizzivy_maintenance LOGIN CONNECTION LIMIT 2;
  END IF;
END
$$;

-- A role that exists is changed only where it differs. CREATE ROLE already says
-- NOSUPERUSER NOCREATEDB NOCREATEROLE, and a managed provider's admin is not a
-- superuser: it may not name the SUPERUSER attribute at all, and may name
-- CREATEDB only if it holds it. So a role that is already right is not altered,
-- and one that drifted is altered by the attributes that drifted.
DO $$
DECLARE
  drift text;
BEGIN
  SELECT concat_ws(' ',
           CASE WHEN rolsuper THEN 'NOSUPERUSER' END,
           CASE WHEN rolcreatedb THEN 'NOCREATEDB' END,
           CASE WHEN rolcreaterole THEN 'NOCREATEROLE' END,
           CASE WHEN NOT rolcanlogin THEN 'LOGIN' END,
           CASE WHEN rolconnlimit <> 2 THEN 'CONNECTION LIMIT 2' END)
    INTO drift
    FROM pg_roles
   WHERE rolname = 'quizzivy_maintenance';
  IF drift <> '' THEN
    EXECUTE 'ALTER ROLE quizzivy_maintenance ' || drift;
  END IF;
END
$$;
SQL

# Passed as a psql variable, not interpolated. :'pw' quotes and escapes as a SQL
# literal, so a password containing an apostrophe is a password rather than the
# end of the string -- and a chosen value is not statement injection into a
# session running as the superuser. The same technique cmd/seedadmin documents
# for the bootstrap password hash.
set_password() {
  run_psql postgres -v role="$1" -v pw="$2" <<'SQL'
ALTER ROLE :"role" PASSWORD :'pw';
SQL
}

CHANGED_PASSWORDS=""
for entry in "quizzivy_migrate:$migrate_pw" "quizzivy_app:$app_pw" "quizzivy_maintenance:$maintenance_pw"; do
  role="${entry%%:*}"
  password="${entry#*:}"
  if [ -n "$password" ]; then
    set_password "$role" "$password"
    CHANGED_PASSWORDS="${CHANGED_PASSWORDS} ${role}"
  fi
done

# CREATEDB is for the test suite, not for production.
#
# TestMigrationsAreReversible drops the whole schema to prove every Down works.
# It used to do that in the shared test database, which "go test ./..." runs
# packages against IN PARALLEL -- so it deleted app.users out from under
# internal/classes mid-query and turned develop CI red for 12 straight runs.
# The test now creates a scratch database, does its damage there, and drops it,
# which needs this. A managed database is not a local run and does not get it.
#
# The heredocs above are QUOTED (<<'SQL'), so nothing in them is shell text: the
# passwords arrive as psql variables instead of being expanded in. That was not
# always true, and both failure modes were real. While one was unquoted, a
# backtick in a comment made provisioning run the test suite and pipe its
# output into psql; the fix for that used a dollar-brace and got "bad
# substitution". Quoting removes the hazard rather than asking each editor to
# remember it.
if [ "$LOCAL" = yes ]; then
  run_psql postgres -c "ALTER ROLE quizzivy_migrate CREATEDB"
fi

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

run_psql postgres -c "GRANT CONNECT ON DATABASE ${DB_NAME} TO quizzivy_app, quizzivy_maintenance"


# Deny the app role anything in public; everything it needs is in schema app,
# granted by migration 00022.
run_psql "${DB_NAME}" <<'SQL'
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO quizzivy_migrate, quizzivy_app;
SQL

echo "quizzivy: roles and database ready"
if [ -n "$CHANGED_PASSWORDS" ]; then
  echo "quizzivy: passwords set for:${CHANGED_PASSWORDS}"
fi
for role in quizzivy_migrate quizzivy_app quizzivy_maintenance; do
  case "$CHANGED_PASSWORDS" in
    *"$role"*) ;;
    *) echo "quizzivy: password of $role left as it was" ;;
  esac
done
