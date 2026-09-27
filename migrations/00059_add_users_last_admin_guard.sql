-- +goose Up

-- The last active Admin can be neither demoted, disabled nor deleted, whatever
-- the path: R5's commands, a bulk action or a hand-written UPDATE (plan 70
-- §4.3). Only a change that takes an active Admin away locks anything: the FOR
-- UPDATE on the Admin role row serialises concurrent demotions, and the count
-- after it runs on a fresh READ COMMITTED snapshot, so of two transactions
-- disabling the only two Admins exactly one commits.
--
-- SECURITY DEFINER, owned by quizzivy_migrate: the FOR UPDATE needs UPDATE
-- privilege on app.roles, which 00054 revokes from the app role, and as the
-- invoker a demotion would fail with 42501 instead of this guard. It is the
-- schema's first definer function; search_path is pinned so it resolves only
-- app and pg_catalog objects.
-- +goose StatementBegin
CREATE FUNCTION app.users_last_admin() RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = app, pg_catalog
AS $fn$
DECLARE
  admin_role uuid;
BEGIN
  SELECT r.id INTO admin_role FROM app.roles r WHERE r.builtin_key = 'admin';
  IF OLD.role_id IS DISTINCT FROM admin_role OR OLD.disabled_at IS NOT NULL THEN
    RETURN NULL;
  END IF;
  IF TG_OP = 'UPDATE' AND NEW.role_id = admin_role AND NEW.disabled_at IS NULL THEN
    RETURN NULL;
  END IF;
  PERFORM 1 FROM app.roles r WHERE r.id = admin_role FOR UPDATE;
  IF NOT EXISTS (SELECT 1 FROM app.users u WHERE u.role_id = admin_role AND u.disabled_at IS NULL) THEN
    RAISE EXCEPTION 'the last active Admin cannot be demoted, disabled or deleted'
      USING ERRCODE = 'check_violation', CONSTRAINT = 'users_last_admin';
  END IF;
  RETURN NULL;
END;
$fn$;
-- +goose StatementEnd

-- role is listed for the v0.7.0 binary, which writes role and lets
-- users_sync_legacy_role derive role_id: a column list matches the columns an
-- UPDATE names, not the ones a BEFORE trigger changes.
CREATE TRIGGER users_last_admin AFTER UPDATE OF role, role_id, disabled_at OR DELETE ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.users_last_admin();

-- +goose Down

DROP TRIGGER users_last_admin ON app.users;
DROP FUNCTION app.users_last_admin();
