-- +goose Up

-- users.role_id, the expand half (T-R2.2, plan 70 §3). The v0.7.0 binary keeps
-- writing users.role while this one writes role_id, so a trigger keeps the two
-- in step until R3 validates the constraint and drops role.
ALTER TABLE app.users ADD COLUMN role_id uuid REFERENCES app.roles ON DELETE RESTRICT;

-- The backfill is not a change a user made, so updated_at must not move.
ALTER TABLE app.users DISABLE TRIGGER users_set_updated_at;

UPDATE app.users u
   SET role_id = r.id
  FROM app.roles r
 WHERE r.builtin_key = u.role::text
   AND u.role_id IS NULL;

ALTER TABLE app.users ENABLE TRIGGER users_set_updated_at;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.users WHERE role_id IS NULL) THEN
    RAISE EXCEPTION 'users.role_id backfill left % user(s) without a role',
      (SELECT count(*) FROM app.users WHERE role_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

-- PG18's NOT NULL ... NOT VALID (AGENTS, verified platform facts): enforced
-- for every new and updated row now, validated when R3 contracts.
ALTER TABLE app.users ADD CONSTRAINT users_role_id_not_null NOT NULL role_id NOT VALID;

-- An insert without role_id is the old binary's: it takes the role from role.
-- An insert or update with role_id derives role from it, over the column's
-- default: 'student' for a student-like role, 'admin' for any other. An update
-- of role alone re-derives role_id.
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

-- +goose Down

DROP TRIGGER users_sync_legacy_role ON app.users;
DROP FUNCTION app.users_sync_legacy_role();
ALTER TABLE app.users DROP CONSTRAINT users_role_id_not_null;
ALTER TABLE app.users DROP COLUMN role_id;
