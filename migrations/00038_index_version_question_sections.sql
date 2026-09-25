-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY tvq_section_identity_key
  ON app.test_version_questions (id, test_version_section_id);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS app.tvq_section_identity_key;
