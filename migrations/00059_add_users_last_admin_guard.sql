-- +goose Up

-- The last active Admin can be neither demoted, disabled nor deleted, whatever
-- the path: R5's commands, a bulk action or a hand-written UPDATE (plan 70
-- §4.3). Only a change that takes an active Admin away locks anything: FOR NO
-- KEY UPDATE on the Admin role row serialises concurrent departures without
-- conflicting with the FOR KEY SHARE that foreign-key checks on users.role_id
-- take, and the count after it runs on a fresh READ COMMITTED snapshot, so of
-- two transactions disabling the only two Admins exactly one commits. That
-- holds under READ COMMITTED, which every writer in the app uses, and under
-- SERIALIZABLE; two REPEATABLE READ transactions count on their own snapshots
-- and could both pass. Locking the remaining Admins instead would make two
-- departures that leave a third Admin deadlock, which is the likelier case.
--
-- SECURITY DEFINER, owned by quizzivy_migrate: the row lock needs UPDATE on
-- app.roles, which 00054 revokes from the app role, and as the invoker a
-- demotion would fail with 42501 instead of this guard. It is the schema's
-- first definer function, so it is hardened as the PostgreSQL docs require:
-- search_path lists pg_temp explicitly and last, so no temporary relation or
-- type a caller creates is resolved inside it; every type and relation is
-- schema-qualified; and EXECUTE is revoked from PUBLIC, so no one can attach it
-- to a table of their own. A trigger fires whatever EXECUTE says.
-- +goose StatementBegin
CREATE FUNCTION app.users_last_admin() RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, app, pg_temp
AS $fn$
DECLARE
  admin_role pg_catalog.uuid;
BEGIN
  SELECT r.id INTO admin_role FROM app.roles r WHERE r.builtin_key = 'admin';
  IF OLD.role_id IS DISTINCT FROM admin_role OR OLD.disabled_at IS NOT NULL THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'UPDATE' AND NEW.role_id = admin_role AND NEW.disabled_at IS NULL THEN
    RETURN NULL;
  END IF;
  PERFORM 1 FROM app.roles r WHERE r.id = admin_role FOR NO KEY UPDATE;
  IF NOT EXISTS (SELECT 1 FROM app.users u WHERE u.role_id = admin_role AND u.disabled_at IS NULL) THEN
    RAISE EXCEPTION 'the last active Admin cannot be demoted, disabled or deleted'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'users_last_admin';
  END IF;
  RETURN NULL;
END;
$fn$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app.users_last_admin() FROM PUBLIC;

-- No column list. The v0.7.0 binary demotes by writing role, and a column list
-- matches the columns an UPDATE names, not those users_sync_legacy_role sets;
-- naming role would also make R3's DROP COLUMN role depend on this trigger.
-- Every other update returns at the function's first test.
CREATE TRIGGER users_last_admin AFTER UPDATE OR DELETE ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.users_last_admin();

-- +goose Down

DROP TRIGGER users_last_admin ON app.users;
DROP FUNCTION app.users_last_admin();
