-- +goose NO TRANSACTION
-- +goose Up

-- The accounts a staff member created (T-R2.12d), and the ON DELETE SET NULL
-- scan when a creator is deleted. CONCURRENTLY, because the table is populated
-- and in use; one index per file, so an interrupted build leaves at most this
-- one behind (T-R2.10).
CREATE INDEX CONCURRENTLY users_created_by_idx
  ON app.users (created_by)
  WHERE created_by IS NOT NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.users_created_by_idx;
