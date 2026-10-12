-- +goose Up

-- T-R3.1, the contract half of 00056. users.role_id has been the only role the
-- application reads or writes since v0.8.0, and the previous binary (v0.9.0)
-- never names users.role, so the column, the trigger that kept the two in step
-- and the enum behind them go.
--
-- Lock. The constraint validates under SHARE UPDATE EXCLUSIVE, which blocks
-- neither a read nor a write of app.users. The drops then take ACCESS EXCLUSIVE
-- and hold it to commit. Nothing is rewritten, because dropping a column only
-- marks it, so that window is milliseconds on any table size. A query already
-- running on app.users delays the drops, and every later query on the table
-- queues behind them, so lock_timeout turns a lock that does not arrive within
-- five seconds into a failed migration: the release stops with the schema as it
-- was, rather than holding every sign-in behind a lock request.
--
-- Order. VALIDATE comes first, so a user without a role stops the transaction
-- before anything is dropped. The trigger goes before the column because its
-- UPDATE OF role list depends on the column. No IF EXISTS anywhere: a wrong
-- name must fail the migration, not leave an object behind.
SET LOCAL lock_timeout = '5s';

ALTER TABLE app.users VALIDATE CONSTRAINT users_role_id_not_null;

DROP TRIGGER users_sync_legacy_role ON app.users;
DROP FUNCTION app.users_sync_legacy_role();
DROP INDEX app.users_role_active_idx;
ALTER TABLE app.users DROP COLUMN role;
DROP TYPE app.user_role;

-- +goose Down

-- Restores the column the way 00004 and 00056 left it. The legacy value is
-- derived from what the role holds, through the view that is the single strict
-- student predicate (00055): a grants-only rule would classify the Admin, which
-- stores at most its "Take tests" cell, as a student. The backfill is not a
-- change a user made, so updated_at must not move. users_role_id_not_null stays
-- validated: Down never weakens a valid NOT NULL.
SET LOCAL lock_timeout = '5s';

CREATE TYPE app.user_role AS ENUM ('admin', 'student');
GRANT USAGE ON TYPE app.user_role TO quizzivy_app;

ALTER TABLE app.users ADD COLUMN role app.user_role;

ALTER TABLE app.users DISABLE TRIGGER users_set_updated_at;

UPDATE app.users
   SET role = CASE
         WHEN role_id IN (SELECT id FROM app.student_like_roles) THEN 'student'::app.user_role
         ELSE 'admin'::app.user_role
       END;

ALTER TABLE app.users ENABLE TRIGGER users_set_updated_at;

ALTER TABLE app.users ALTER COLUMN role SET DEFAULT 'student';
ALTER TABLE app.users ALTER COLUMN role SET NOT NULL;

CREATE INDEX users_role_active_idx ON app.users (role) WHERE disabled_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION app.users_sync_legacy_role() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  IF TG_OP = 'INSERT' AND NEW.role_id IS NULL THEN
    NEW.role_id := (SELECT r.id FROM app.roles r WHERE r.builtin_key = NEW.role::text);
  ELSIF TG_OP = 'INSERT' OR NEW.role_id IS DISTINCT FROM OLD.role_id THEN
    NEW.role := CASE
      WHEN EXISTS (SELECT 1 FROM app.student_like_roles s WHERE s.id = NEW.role_id)
        THEN 'student'::app.user_role
      ELSE 'admin'::app.user_role
    END;
  ELSIF NEW.role IS DISTINCT FROM OLD.role THEN
    NEW.role_id := (SELECT r.id FROM app.roles r WHERE r.builtin_key = NEW.role::text);
  END IF;
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

CREATE TRIGGER users_sync_legacy_role BEFORE INSERT OR UPDATE OF role, role_id ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.users_sync_legacy_role();
