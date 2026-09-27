-- +goose Up

-- classes.teacher_id, the expand half (T-R2.9, plan 70 §3). app.classes has
-- never recorded who created a class, so the backfill uses the rule v0.7.0
-- already shows every class under: the oldest active Admin. In production that
-- is the one staff account, so no class's teacher changes. Provenance for
-- classes created from R2 on is the class.created audit row, and R5's transfer
-- moves teacher_id alone (D-23). R3 validates the constraint and drops the fill
-- trigger.
ALTER TABLE app.classes ADD COLUMN teacher_id uuid REFERENCES app.users ON DELETE RESTRICT;

-- The backfill is not a change a user made, so updated_at must not move.
ALTER TABLE app.classes DISABLE TRIGGER classes_set_updated_at;

UPDATE app.classes
   SET teacher_id = (SELECT u.id
                       FROM app.users u
                       JOIN app.roles r ON r.id = u.role_id
                      WHERE r.builtin_key = 'admin' AND u.disabled_at IS NULL
                      ORDER BY u.created_at, u.id
                      LIMIT 1)
 WHERE teacher_id IS NULL;

ALTER TABLE app.classes ENABLE TRIGGER classes_set_updated_at;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.classes WHERE teacher_id IS NULL) THEN
    RAISE EXCEPTION 'classes.teacher_id backfill left % class(es) without a teacher: no active Admin',
      (SELECT count(*) FROM app.classes WHERE teacher_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.classes ADD CONSTRAINT classes_teacher_id_not_null NOT NULL teacher_id NOT VALID;

-- The v0.7.0 binary inserts without teacher_id and shows the oldest active
-- Admin as every class's teacher; this binary names the acting user.
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

-- +goose Down

DROP TRIGGER classes_fill_teacher ON app.classes;
DROP FUNCTION app.classes_fill_teacher();
ALTER TABLE app.classes DROP CONSTRAINT classes_teacher_id_not_null;
ALTER TABLE app.classes DROP COLUMN teacher_id;
