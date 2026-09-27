-- +goose Up

-- question_groups.owner_id, the expand half (T-R2.9, plan 70 §3), as 00060 does for
-- tests: created_by stays the provenance (D-22). R3 validates the constraint
-- and drops the fill trigger.
-- No REFERENCES here: 00068 adds the foreign key in its own transaction.
ALTER TABLE app.question_groups ADD COLUMN owner_id uuid;

-- The backfill is not a change a user made, so updated_at, which the lists
-- sort and show, must not move.
ALTER TABLE app.question_groups DISABLE TRIGGER question_groups_set_updated_at;

UPDATE app.question_groups SET owner_id = created_by WHERE owner_id IS NULL;

ALTER TABLE app.question_groups ENABLE TRIGGER question_groups_set_updated_at;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.question_groups WHERE owner_id IS NULL) THEN
    RAISE EXCEPTION 'question_groups.owner_id backfill left % question group(s) without an owner',
      (SELECT count(*) FROM app.question_groups WHERE owner_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.question_groups ADD CONSTRAINT question_groups_owner_id_not_null NOT NULL owner_id NOT VALID;

CREATE TRIGGER question_groups_fill_owner BEFORE INSERT ON app.question_groups
  FOR EACH ROW EXECUTE FUNCTION app.fill_owner_from_created_by();

-- +goose Down

DROP TRIGGER question_groups_fill_owner ON app.question_groups;
ALTER TABLE app.question_groups DROP CONSTRAINT question_groups_owner_id_not_null;
ALTER TABLE app.question_groups DROP COLUMN owner_id;
