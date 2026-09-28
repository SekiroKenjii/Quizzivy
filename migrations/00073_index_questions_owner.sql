-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's question bank, newest first (T-R2.12b). Group members are not
-- bank rows, so they stay out of the index. CONCURRENTLY, because the table is
-- populated and in use; one index per file, so an interrupted build leaves at
-- most this one behind (T-R2.10).
CREATE INDEX CONCURRENTLY questions_owner_bank_idx
  ON app.questions (owner_id, id DESC)
  WHERE deleted_at IS NULL AND context_group_id IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.questions_owner_bank_idx;
