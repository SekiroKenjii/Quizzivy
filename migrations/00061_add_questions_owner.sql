-- +goose Up

-- questions.owner_id, the expand half (T-R2.9, plan 70 §3), as 00060 does for
-- tests: created_by stays the provenance (D-22). R3 validates the constraint
-- and drops the fill trigger.
ALTER TABLE app.questions ADD COLUMN owner_id uuid REFERENCES app.users ON DELETE RESTRICT;

-- The backfill is not a change a user made, so updated_at, which the lists
-- sort and show, must not move.
ALTER TABLE app.questions DISABLE TRIGGER questions_set_updated_at;

UPDATE app.questions SET owner_id = created_by WHERE owner_id IS NULL;

ALTER TABLE app.questions ENABLE TRIGGER questions_set_updated_at;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE owner_id IS NULL) THEN
    RAISE EXCEPTION 'questions.owner_id backfill left % question(s) without an owner',
      (SELECT count(*) FROM app.questions WHERE owner_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.questions ADD CONSTRAINT questions_owner_id_not_null NOT NULL owner_id NOT VALID;

CREATE TRIGGER questions_fill_owner BEFORE INSERT ON app.questions
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_created_by();

-- +goose Down

DROP TRIGGER questions_fill_owner ON app.questions;
ALTER TABLE app.questions DROP CONSTRAINT questions_owner_id_not_null;
ALTER TABLE app.questions DROP COLUMN owner_id;
