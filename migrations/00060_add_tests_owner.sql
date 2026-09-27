-- +goose Up

-- tests.owner_id, the expand half (T-R2.9, plan 70 §3). created_by stays the
-- provenance and owner_id is who the test belongs to, so R5's ownership
-- transfer and R7's co-editing move a test without rewriting who wrote it
-- (D-22). R3 validates the constraint and drops the fill trigger.
-- No REFERENCES here: 00066 adds the foreign key in its own transaction.
ALTER TABLE app.tests ADD COLUMN owner_id uuid;

-- The backfill is not a change a user made, so updated_at must not move: an
-- open builder tab sends it back as expectedUpdatedAt, and a moved value would
-- fail its next autosave with STALE_WRITE during the deploy.
ALTER TABLE app.tests DISABLE TRIGGER tests_set_updated_at;

UPDATE app.tests SET owner_id = created_by WHERE owner_id IS NULL;

ALTER TABLE app.tests ENABLE TRIGGER tests_set_updated_at;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.tests WHERE owner_id IS NULL) THEN
    RAISE EXCEPTION 'tests.owner_id backfill left % test(s) without an owner',
      (SELECT count(*) FROM app.tests WHERE owner_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

-- PG18's NOT NULL ... NOT VALID (AGENTS, verified platform facts): enforced
-- for every new and updated row now, validated when R3 contracts.
ALTER TABLE app.tests ADD CONSTRAINT tests_owner_id_not_null NOT NULL owner_id NOT VALID;

-- The v0.7.0 binary inserts without owner_id; this binary always names it. The
-- questions and question_groups files reuse the function.
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

-- +goose Down

DROP TRIGGER tests_fill_owner ON app.tests;
DROP FUNCTION app.fill_owner_from_created_by();
ALTER TABLE app.tests DROP CONSTRAINT tests_owner_id_not_null;
ALTER TABLE app.tests DROP COLUMN owner_id;
