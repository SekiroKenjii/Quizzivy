-- +goose Up
ALTER TABLE app.test_versions
  ADD COLUMN delivery_version text NOT NULL DEFAULT 'section_v1'
    CHECK (delivery_version IN ('section_v1','group_v1'));

UPDATE app.test_versions v SET delivery_version = 'group_v1'
WHERE EXISTS (
  SELECT 1 FROM app.test_version_sections s
  JOIN app.test_version_groups g ON g.test_version_section_id = s.id
  WHERE s.test_version_id = v.id
);

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.test_versions WHERE delivery_version <> 'section_v1') THEN
    RAISE EXCEPTION 'delivery version cannot be removed after group-aware publication';
  END IF;
END;
$$;
-- +goose StatementEnd
ALTER TABLE app.test_versions DROP COLUMN delivery_version;
