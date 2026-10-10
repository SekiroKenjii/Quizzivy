-- R4 (v0.10.0) rehearsal of 00105_drop_users_legacy_role.sql on a Neon branch
-- of production.
--
-- Run as quizzivy_migrate, twice:
--   psql "$BRANCH_DSN" -v before=1 -f docs/setup/rehearsals/r4-legacy-role.sql > before.txt
--   goose up (time it)
--   psql "$BRANCH_DSN" -f docs/setup/rehearsals/r4-legacy-role.sql > after.txt
-- Section A must print the same in both files. Section B runs only after
-- goose up; every "must be" line must print what it says. Record the time of
-- goose up itself in the release PR, then delete the branch. Query timing is
-- off, so a plain diff of the two files shows section A unchanged.
-- Read-only: nothing here writes.

\set ON_ERROR_STOP on
\pset footer off
\timing off

\echo '== A. Unchanged by the migration (compare before.txt with after.txt)'

\echo '-- A1. Users by role and whether they are disabled'
SELECT r.builtin_key, (u.disabled_at IS NOT NULL) AS disabled, count(*) AS users
  FROM app.users u
  JOIN app.roles r ON r.id = u.role_id
 GROUP BY 1, 2
 ORDER BY 1, 2;

\echo '-- A2. Rows and the latest updated_at of users'
SELECT count(*) AS row_count, max(updated_at) AS max_updated_at FROM app.users;

\if :{?before}
\echo '-- A3. Users whose legacy role disagrees with what their role holds (must be 0)'
SELECT count(*) AS must_be_0
  FROM app.users u
 WHERE (u.role = 'student') IS DISTINCT FROM (u.role_id IN (SELECT id FROM app.student_like_roles));

\echo '== Before goose up: section A only.'
\quit
\endif

\echo '== B. After goose up'

\echo '-- B1. The column users.role (must be 0)'
SELECT count(*) AS must_be_0
  FROM pg_attribute
 WHERE attrelid = 'app.users'::regclass AND attname = 'role' AND NOT attisdropped;

\echo '-- B2. The enum, the function, the trigger and the index (must be 0)'
SELECT (SELECT count(*) FROM pg_type WHERE typname = 'user_role' AND typnamespace = 'app'::regnamespace) AS enum_must_be_0,
       (SELECT count(*) FROM pg_proc WHERE proname = 'users_sync_legacy_role' AND pronamespace = 'app'::regnamespace) AS function_must_be_0,
       (SELECT count(*) FROM pg_trigger WHERE tgname = 'users_sync_legacy_role') AS trigger_must_be_0,
       (SELECT count(*) FROM pg_class WHERE relname = 'users_role_active_idx' AND relnamespace = 'app'::regnamespace) AS index_must_be_0;

\echo '-- B3. users_role_id_not_null is validated (must be true)'
SELECT convalidated AS must_be_true
  FROM pg_constraint
 WHERE conname = 'users_role_id_not_null' AND conrelid = 'app.users'::regclass;

\echo '-- B4. Users without role_id (must be 0)'
SELECT count(*) AS must_be_0 FROM app.users WHERE role_id IS NULL;

\echo '-- B5. The triggers that stay are enabled (must list users_last_admin and users_set_updated_at, both O)'
SELECT tgname, tgenabled
  FROM pg_trigger
 WHERE tgrelid = 'app.users'::regclass AND NOT tgisinternal
 ORDER BY tgname;

\echo '-- B6. Not-validated constraints left in app (the owner constraints until the ownership step ships)'
SELECT conrelid::regclass AS table_name, conname
  FROM pg_constraint
 WHERE connamespace = 'app'::regnamespace AND NOT convalidated
 ORDER BY 1, 2;
