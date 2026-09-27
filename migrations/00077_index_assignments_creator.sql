-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's assignments, newest first (T-R2.12e). CONCURRENTLY, because the
-- table is populated and in use; one index per file, so an interrupted build
-- leaves at most this one behind (T-R2.10).
CREATE INDEX CONCURRENTLY assignments_creator_idx
  ON app.assignments (created_by, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.assignments_creator_idx;
