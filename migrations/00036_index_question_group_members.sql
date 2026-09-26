-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY questions_context_identity_key
  ON app.questions (context_group_id, id);
CREATE UNIQUE INDEX CONCURRENTLY questions_context_ordinal_key
  ON app.questions (context_group_id, context_ordinal);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS app.questions_context_ordinal_key;
DROP INDEX CONCURRENTLY IF EXISTS app.questions_context_identity_key;
