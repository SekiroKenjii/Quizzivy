-- R4 (v0.10.0) rehearsal of 00106_validate_ownership.sql on a Neon branch of
-- production.
--
-- Run as quizzivy_migrate, twice:
--   psql "$BRANCH_DSN" -v before=1 -f docs/setup/rehearsals/r4-ownership.sql > before.txt
--   goose up (time it)
--   psql "$BRANCH_DSN" -f docs/setup/rehearsals/r4-ownership.sql > after.txt
-- Section A must print the same in both files. Section P runs only before
-- goose up and says whether the migration will stop. Section B runs only after
-- goose up; every "must be" line must print what it says. Record the time of
-- goose up itself in the release PR, then delete the branch. Query timing is
-- off, so a plain diff of the two files shows section A unchanged.
--
-- The cost of the validation scans is a separate run, because it prints
-- timings that differ from one run to the next:
--   psql "$BRANCH_DSN" -v before=1 -v scan=1 -f docs/setup/rehearsals/r4-ownership.sql
-- Read-only: nothing here writes.

\set ON_ERROR_STOP on
\pset footer off
\timing off

\echo '== A. Unchanged by the migration (compare before.txt with after.txt)'

\echo '-- A1. Rows per table'
SELECT 'classes' AS table_name, count(*) AS row_count FROM app.classes
UNION ALL SELECT 'tests', count(*) FROM app.tests
UNION ALL SELECT 'questions', count(*) FROM app.questions
UNION ALL SELECT 'question_groups', count(*) FROM app.question_groups
UNION ALL SELECT 'media_assets', count(*) FROM app.media_assets
ORDER BY 1;

\echo '-- A2. Rows whose owner is not the person who made them (an ownership transfer, or a class)'
SELECT 'tests' AS table_name, count(*) FILTER (WHERE owner_id <> created_by) AS owner_differs FROM app.tests
UNION ALL SELECT 'questions', count(*) FILTER (WHERE owner_id <> created_by) FROM app.questions
UNION ALL SELECT 'question_groups', count(*) FILTER (WHERE owner_id <> created_by) FROM app.question_groups
UNION ALL SELECT 'media_assets', count(*) FILTER (WHERE owner_id <> uploaded_by) FROM app.media_assets
ORDER BY 1;

\echo '-- A3. Owners per table'
SELECT 'classes' AS table_name, count(DISTINCT teacher_id) AS owners FROM app.classes
UNION ALL SELECT 'tests', count(DISTINCT owner_id) FROM app.tests
UNION ALL SELECT 'questions', count(DISTINCT owner_id) FROM app.questions
UNION ALL SELECT 'question_groups', count(DISTINCT owner_id) FROM app.question_groups
UNION ALL SELECT 'media_assets', count(DISTINCT owner_id) FROM app.media_assets
ORDER BY 1;

\echo '-- A4. The latest updated_at of the tables that have one'
SELECT 'classes' AS table_name, max(updated_at) AS max_updated_at FROM app.classes
UNION ALL SELECT 'tests', max(updated_at) FROM app.tests
UNION ALL SELECT 'questions', max(updated_at) FROM app.questions
UNION ALL SELECT 'question_groups', max(updated_at) FROM app.question_groups
ORDER BY 1;

\echo '-- A5. The triggers that stay on these tables are enabled (every row O)'
SELECT tgrelid::regclass AS table_name, tgname, tgenabled
  FROM pg_trigger
 WHERE tgrelid IN ('app.classes'::regclass, 'app.tests'::regclass, 'app.questions'::regclass,
                   'app.question_groups'::regclass, 'app.media_assets'::regclass)
   AND NOT tgisinternal AND tgname !~ '_fill_'
 ORDER BY 1, 2;

\if :{?before}
\echo '== P. Before goose up: will the migration stop?'

\echo '-- P1. Rows without an owner (must be 0 in every row; any other value stops the migration)'
SELECT 'classes' AS table_name, count(*) AS must_be_0 FROM app.classes WHERE teacher_id IS NULL
UNION ALL SELECT 'tests', count(*) FROM app.tests WHERE owner_id IS NULL
UNION ALL SELECT 'questions', count(*) FROM app.questions WHERE owner_id IS NULL
UNION ALL SELECT 'question_groups', count(*) FROM app.question_groups WHERE owner_id IS NULL
UNION ALL SELECT 'media_assets', count(*) FROM app.media_assets WHERE owner_id IS NULL
ORDER BY 1;

\echo '-- P2. The five constraints exist and are not validated yet (must be five rows, false)'
SELECT conrelid::regclass AS table_name, conname, convalidated
  FROM pg_constraint
 WHERE connamespace = 'app'::regnamespace
   AND conname IN ('classes_teacher_id_not_null', 'tests_owner_id_not_null', 'questions_owner_id_not_null',
                   'question_groups_owner_id_not_null', 'media_assets_owner_id_not_null')
 ORDER BY 1;

\echo '-- P3. The fill triggers and their functions exist (must be five triggers, three functions)'
SELECT (SELECT count(*) FROM pg_trigger WHERE tgname IN ('tests_fill_owner', 'questions_fill_owner',
          'question_groups_fill_owner', 'media_assets_fill_owner', 'classes_fill_teacher')) AS triggers,
       (SELECT count(*) FROM pg_proc WHERE pronamespace = 'app'::regnamespace
          AND proname IN ('fill_owner_from_created_by', 'fill_owner_from_uploaded_by', 'classes_fill_teacher')) AS functions;

\if :{?scan}
\echo '== C. The validation scan of each table (the cost of VALIDATE CONSTRAINT; timings differ per run)'
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) SELECT 1 FROM app.classes WHERE teacher_id IS NULL LIMIT 1;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) SELECT 1 FROM app.tests WHERE owner_id IS NULL LIMIT 1;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) SELECT 1 FROM app.questions WHERE owner_id IS NULL LIMIT 1;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) SELECT 1 FROM app.question_groups WHERE owner_id IS NULL LIMIT 1;
EXPLAIN (ANALYZE, BUFFERS, SETTINGS) SELECT 1 FROM app.media_assets WHERE owner_id IS NULL LIMIT 1;
\endif

\echo '== Before goose up: sections A and P only.'
\quit
\endif

\echo '== B. After goose up'

\echo '-- B1. The five constraints are validated (must be five rows, true)'
SELECT conrelid::regclass AS table_name, conname, convalidated AS must_be_true
  FROM pg_constraint
 WHERE connamespace = 'app'::regnamespace
   AND conname IN ('classes_teacher_id_not_null', 'tests_owner_id_not_null', 'questions_owner_id_not_null',
                   'question_groups_owner_id_not_null', 'media_assets_owner_id_not_null')
 ORDER BY 1;

\echo '-- B2. The fill triggers and their functions are gone (must be 0 and 0)'
SELECT (SELECT count(*) FROM pg_trigger WHERE tgname IN ('tests_fill_owner', 'questions_fill_owner',
          'question_groups_fill_owner', 'media_assets_fill_owner', 'classes_fill_teacher')) AS triggers_must_be_0,
       (SELECT count(*) FROM pg_proc WHERE pronamespace = 'app'::regnamespace
          AND proname IN ('fill_owner_from_created_by', 'fill_owner_from_uploaded_by', 'classes_fill_teacher')) AS functions_must_be_0;

\echo '-- B3. Each owner column is NOT NULL (must be five rows, true)'
SELECT a.attrelid::regclass AS table_name, a.attname, a.attnotnull AS must_be_true
  FROM pg_attribute a
 WHERE (a.attrelid, a.attname) IN (('app.classes'::regclass, 'teacher_id'), ('app.tests'::regclass, 'owner_id'),
        ('app.questions'::regclass, 'owner_id'), ('app.question_groups'::regclass, 'owner_id'),
        ('app.media_assets'::regclass, 'owner_id'))
   AND NOT a.attisdropped
 ORDER BY 1;

\echo '-- B4. The foreign keys to users are still there and validated (must be five rows, r, true)'
SELECT conrelid::regclass AS table_name, conname, confdeltype, convalidated
  FROM pg_constraint
 WHERE connamespace = 'app'::regnamespace
   AND conname IN ('classes_teacher_id_fkey', 'tests_owner_id_fkey', 'questions_owner_id_fkey',
                   'question_groups_owner_id_fkey', 'media_assets_owner_id_fkey')
 ORDER BY 1;

\echo '-- B5. Not-validated constraints left in app (must be empty once 00105 has run too)'
SELECT conrelid::regclass AS table_name, conname
  FROM pg_constraint
 WHERE connamespace = 'app'::regnamespace AND NOT convalidated
 ORDER BY 1, 2;
