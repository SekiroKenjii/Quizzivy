-- +goose Up

-- T-R3.2, the contract half of 00060 to 00065. Every insert path has named its
-- owner since v0.8.0, and the previous binary (v0.9.0) does too, so the five
-- owner constraints are validated and the BEFORE INSERT triggers that filled an
-- owner for the v0.7.0 binary go, with their functions.
--
-- Lock. Each VALIDATE CONSTRAINT takes SHARE UPDATE EXCLUSIVE and scans its
-- table once, so reads and writes continue. The drops take ACCESS EXCLUSIVE on
-- their table and hold it to commit; they come last and touch the catalog
-- only, so the window is milliseconds at any table size. A query already
-- running on a table delays its drop, and every later query on that table
-- queues behind the request, so lock_timeout turns a lock that does not arrive
-- within five seconds into a failed migration: the release stops with the
-- schema as it was, rather than holding every request behind a lock request.
--
-- Order. The check comes first so that a row without an owner stops the
-- transaction, naming its table, before anything changes. It cannot be beaten
-- by a concurrent insert: the NOT VALID constraints have refused a NULL owner
-- for every new and updated row since 00060 to 00065. A trigger goes before its
-- function. No IF EXISTS anywhere: a wrong name must fail the migration, not
-- leave an object behind.
SET LOCAL lock_timeout = '5s';

-- +goose StatementBegin
DO $$
DECLARE
  target_table text;
  target_column text;
  missing bigint;
BEGIN
  FOR target_table, target_column IN
    SELECT v.table_name, v.column_name
      FROM (VALUES
        ('classes', 'teacher_id'),
        ('tests', 'owner_id'),
        ('questions', 'owner_id'),
        ('question_groups', 'owner_id'),
        ('media_assets', 'owner_id')
      ) AS v(table_name, column_name)
  LOOP
    EXECUTE format('SELECT count(*) FROM app.%I WHERE %I IS NULL', target_table, target_column)
       INTO missing;
    IF missing > 0 THEN
      RAISE EXCEPTION 'app.% has % row(s) without %', target_table, missing, target_column
        USING ERRCODE = 'not_null_violation', SCHEMA = 'app', TABLE = target_table, COLUMN = target_column;
    END IF;
  END LOOP;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.classes VALIDATE CONSTRAINT classes_teacher_id_not_null;
ALTER TABLE app.tests VALIDATE CONSTRAINT tests_owner_id_not_null;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_owner_id_not_null;
ALTER TABLE app.question_groups VALIDATE CONSTRAINT question_groups_owner_id_not_null;
ALTER TABLE app.media_assets VALIDATE CONSTRAINT media_assets_owner_id_not_null;

DROP TRIGGER tests_fill_owner ON app.tests;
DROP TRIGGER questions_fill_owner ON app.questions;
DROP TRIGGER question_groups_fill_owner ON app.question_groups;
DROP FUNCTION app.fill_owner_from_created_by();
DROP TRIGGER media_assets_fill_owner ON app.media_assets;
DROP FUNCTION app.fill_owner_from_uploaded_by();
DROP TRIGGER classes_fill_teacher ON app.classes;
DROP FUNCTION app.classes_fill_teacher();

-- +goose Down

-- Restores the functions and triggers exactly as 00060, 00063 and 00065 left
-- them, for a binary that inserts without an owner. The constraints stay
-- validated: Down never weakens a valid NOT NULL. Nothing in production runs
-- it, and the rows already written keep the owners they were given.
SET LOCAL lock_timeout = '5s';

-- +goose StatementBegin
CREATE FUNCTION app.fill_owner_from_created_by() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.owner_id IS NULL THEN
    NEW.owner_id := NEW.created_by;
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER tests_fill_owner BEFORE INSERT ON app.tests
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_created_by();
CREATE TRIGGER questions_fill_owner BEFORE INSERT ON app.questions
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_created_by();
CREATE TRIGGER question_groups_fill_owner BEFORE INSERT ON app.question_groups
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_created_by();

-- +goose StatementBegin
CREATE FUNCTION app.fill_owner_from_uploaded_by() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.owner_id IS NULL THEN
    NEW.owner_id := NEW.uploaded_by;
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER media_assets_fill_owner BEFORE INSERT ON app.media_assets
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_uploaded_by();

-- +goose StatementBegin
CREATE FUNCTION app.classes_fill_teacher() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.teacher_id IS NULL THEN
    NEW.teacher_id := (SELECT u.id
                         FROM app.users u
                         JOIN app.roles r ON r.id = u.role_id
                        WHERE r.builtin_key = 'admin' AND u.disabled_at IS NULL
                        ORDER BY u.created_at, u.id
                        LIMIT 1);
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER classes_fill_teacher BEFORE INSERT ON app.classes
  FOR EACH ROW EXECUTE FUNCTION app.classes_fill_teacher();
