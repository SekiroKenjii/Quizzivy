-- R2 (v0.8.0) migration rehearsal on a Neon branch of production.
--
-- Run as quizzivy_migrate, twice:
--   psql "$BRANCH_DSN" -v before=1 -f docs/setup/rehearsals/r2.sql > before.txt
--   goose up (time it)
--   psql "$BRANCH_DSN" -f docs/setup/rehearsals/r2.sql > after.txt
-- Section A must print the same in both files. Section B runs only after
-- goose up; every "must be 0" line must print 0. Record the timings and the
-- legacy join-code count in the release PR, then delete the branch.
-- Read-only: nothing here writes.

\set ON_ERROR_STOP on
\pset footer off
\timing on

\echo '== A. Unchanged by the migrations (compare before.txt with after.txt)'

\echo '-- A1. Rows and the latest updated_at of every table that has one (roles is new in R2)'
SELECT format(
         'SELECT %L AS table_name, count(*) AS row_count, max(updated_at) AS max_updated_at FROM app.%I',
         c.table_name, c.table_name)
  FROM information_schema.columns c
  JOIN information_schema.tables t
    ON t.table_schema = c.table_schema AND t.table_name = c.table_name
 WHERE c.table_schema = 'app' AND c.column_name = 'updated_at' AND t.table_type = 'BASE TABLE'
   AND c.table_name <> 'roles'
 ORDER BY c.table_name
\gexec

\echo '-- A2. Users by legacy role, disabled or not'
SELECT role::text, (disabled_at IS NOT NULL) AS disabled, count(*) AS users
  FROM app.users
 GROUP BY 1, 2
 ORDER BY 1, 2;

\echo '-- A3. Join codes: active, revoked, expired'
SELECT count(*) FILTER (WHERE revoked_at IS NULL AND expires_at > now()) AS active,
       count(*) FILTER (WHERE revoked_at IS NOT NULL) AS revoked,
       count(*) FILTER (WHERE revoked_at IS NULL AND expires_at <= now()) AS expired
  FROM app.class_join_codes;

\if :{?before}
\echo '== Before goose up: section A only.'
\quit
\endif

\echo '== B. After goose up'

\echo '-- B1. Users without role_id (must be 0)'
SELECT count(*) AS must_be_0 FROM app.users WHERE role_id IS NULL;

\echo '-- B2. Users by built-in role (the admins of A2 as Admin; the students as Student)'
SELECT r.builtin_key, (u.disabled_at IS NOT NULL) AS disabled, count(*) AS users
  FROM app.users u
  JOIN app.roles r ON r.id = u.role_id
 GROUP BY 1, 2
 ORDER BY 1, 2;

\echo '-- B3. A user whose legacy role disagrees with role_id (must be 0)'
SELECT count(*) AS must_be_0
  FROM app.users u
 WHERE (u.role = 'student') <> (u.role_id IN (SELECT id FROM app.student_like_roles));

\echo '-- B4. NULL owners (each must be 0)'
SELECT (SELECT count(*) FROM app.tests WHERE owner_id IS NULL) AS tests,
       (SELECT count(*) FROM app.questions WHERE owner_id IS NULL) AS questions,
       (SELECT count(*) FROM app.question_groups WHERE owner_id IS NULL) AS question_groups,
       (SELECT count(*) FROM app.media_assets WHERE owner_id IS NULL) AS media_assets,
       (SELECT count(*) FROM app.classes WHERE teacher_id IS NULL) AS classes;

\echo '-- B5. Owners who do not hold the Admin role (must be 0 at release)'
WITH owners(owner_id) AS (
  SELECT owner_id FROM app.tests
  UNION SELECT owner_id FROM app.questions
  UNION SELECT owner_id FROM app.question_groups
  UNION SELECT owner_id FROM app.media_assets
  UNION SELECT teacher_id FROM app.classes
)
SELECT count(*) AS must_be_0
  FROM owners o
  JOIN app.users u ON u.id = o.owner_id
  JOIN app.roles r ON r.id = u.role_id
 WHERE r.builtin_key IS DISTINCT FROM 'admin';

\echo '-- B6. Draft references that cross owners (each must be 0; T-R2.12a refuses them)'
SELECT (SELECT count(*)
          FROM app.test_section_questions sq
          JOIN app.test_sections s ON s.id = sq.test_section_id
          JOIN app.tests t ON t.id = s.test_id
          JOIN app.questions q ON q.id = sq.question_id
         WHERE q.owner_id <> t.owner_id) AS section_questions,
       (SELECT count(*)
          FROM app.test_section_units su
          JOIN app.test_sections s ON s.id = su.test_section_id
          JOIN app.tests t ON t.id = s.test_id
          JOIN app.questions q ON q.id = su.question_id
         WHERE q.owner_id <> t.owner_id) AS question_units,
       (SELECT count(*)
          FROM app.question_groups g
          JOIN app.test_sections s ON s.id = g.owner_section_id
          JOIN app.tests t ON t.id = s.test_id
         WHERE g.owner_id <> t.owner_id) AS section_groups;

\echo '-- B7. Committed imports whose test is not the import creator''s (must be 0; T-R2.12f)'
SELECT count(*) AS must_be_0
  FROM app.word_import_commits c
  JOIN app.word_imports i ON i.id = c.import_id
  JOIN app.tests t ON t.id = c.test_id
 WHERE t.owner_id <> i.created_by;

\echo '-- B8. Active legacy join codes (record: R4 rotates them)'
SELECT count(*) AS active_legacy_codes
  FROM app.class_join_codes
 WHERE lookup_scheme = 1 AND revoked_at IS NULL AND expires_at > now();

\echo '-- B9. The eight new indexes (each must be valid and ready)'
SELECT c.relname AS index_name, i.indisvalid AS valid, i.indisready AS ready
  FROM pg_index i
  JOIN pg_class c ON c.oid = i.indexrelid
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname = 'app'
   AND c.relname IN ('users_role_id_active_idx', 'tests_owner_idx', 'questions_owner_bank_idx',
                     'question_groups_owner_bank_idx', 'classes_teacher_idx',
                     'media_assets_owner_idx', 'users_created_by_idx', 'assignments_creator_idx')
 ORDER BY 1;

\echo '-- B10. Of those, how many exist (must be 8)'
SELECT count(*) AS must_be_8
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE n.nspname = 'app'
   AND c.relname IN ('users_role_id_active_idx', 'tests_owner_idx', 'questions_owner_bank_idx',
                     'question_groups_owner_bank_idx', 'classes_teacher_idx',
                     'media_assets_owner_idx', 'users_created_by_idx', 'assignments_creator_idx');

\echo '-- B11. The catalogue and the built-in roles (22 keys; four roles)'
SELECT (SELECT count(*) FROM app.permissions) AS permission_keys,
       (SELECT count(*) FROM app.roles WHERE builtin_key IS NOT NULL) AS builtin_roles;
