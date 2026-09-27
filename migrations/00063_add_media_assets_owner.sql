-- +goose Up

-- media_assets.owner_id, the expand half (T-R2.9, plan 70 §3), from
-- uploaded_by, which stays the provenance (D-22). The table has no updated_at,
-- so the backfill has no trigger to hold off. R3 validates the constraint and
-- drops the fill trigger.
-- No REFERENCES here: 00069 adds the foreign key in its own transaction.
ALTER TABLE app.media_assets ADD COLUMN owner_id uuid;

UPDATE app.media_assets SET owner_id = uploaded_by WHERE owner_id IS NULL;

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.media_assets WHERE owner_id IS NULL) THEN
    RAISE EXCEPTION 'media_assets.owner_id backfill left % asset(s) without an owner',
      (SELECT count(*) FROM app.media_assets WHERE owner_id IS NULL);
  END IF;
END;
$$;
-- +goose StatementEnd

ALTER TABLE app.media_assets ADD CONSTRAINT media_assets_owner_id_not_null NOT NULL owner_id NOT VALID;

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

-- +goose Down

DROP TRIGGER media_assets_fill_owner ON app.media_assets;
DROP FUNCTION app.fill_owner_from_uploaded_by();
ALTER TABLE app.media_assets DROP CONSTRAINT media_assets_owner_id_not_null;
ALTER TABLE app.media_assets DROP COLUMN owner_id;
