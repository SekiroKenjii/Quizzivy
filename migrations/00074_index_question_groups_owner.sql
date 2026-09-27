-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's bank groups in the order 00049's question_groups_bank_recent_idx
-- serves for everyone (T-R2.12a). CONCURRENTLY, because the table is populated
-- and in use; one index per file, so an interrupted build leaves at most this
-- one behind (T-R2.10).
CREATE INDEX CONCURRENTLY question_groups_owner_bank_idx
  ON app.question_groups (owner_id, updated_at DESC, id DESC)
  WHERE owner_section_id IS NULL AND archived_at IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.question_groups_owner_bank_idx;
