-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's tests list, newest first (T-R2.12a). It is partial, so the
-- owner_id RESTRICT check that every user delete runs, a student's included,
-- cannot use it; like the created_by check beside it, that check scans the
-- table, which is small (§14). CONCURRENTLY, because the table is populated and
-- in use; one index per file, so an interrupted build leaves at most this one
-- behind (T-R2.10).
CREATE INDEX CONCURRENTLY tests_owner_idx
  ON app.tests (owner_id, id DESC)
  WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.tests_owner_idx;
