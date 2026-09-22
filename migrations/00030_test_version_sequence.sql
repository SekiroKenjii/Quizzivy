-- +goose Up

ALTER TABLE app.tests ADD COLUMN last_published_version integer NOT NULL DEFAULT 0;
UPDATE app.tests t
   SET last_published_version = greatest(t.current_version,
       coalesce((SELECT max(v.version) FROM app.test_versions v WHERE v.test_id = t.id), 0));
ALTER TABLE app.tests ADD CONSTRAINT tests_version_sequence_check
  CHECK (last_published_version >= current_version);

-- +goose StatementBegin
CREATE FUNCTION app.keep_test_version_sequence() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.last_published_version := greatest(NEW.last_published_version, NEW.current_version);
  IF TG_OP = 'UPDATE' THEN
    NEW.last_published_version := greatest(NEW.last_published_version, OLD.last_published_version);
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tests_keep_version_sequence BEFORE INSERT OR UPDATE ON app.tests
  FOR EACH ROW EXECUTE FUNCTION app.keep_test_version_sequence();

-- +goose Down

DROP TRIGGER tests_keep_version_sequence ON app.tests;
DROP FUNCTION app.keep_test_version_sequence();
ALTER TABLE app.tests DROP COLUMN last_published_version;
