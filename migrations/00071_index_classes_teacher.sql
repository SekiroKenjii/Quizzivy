-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's own classes (T-R2.12d) and the foreign-key check when a user is
-- deleted. CONCURRENTLY, because the table is populated and in use; one index
-- per file, so an interrupted build leaves at most this one behind (T-R2.10).
CREATE INDEX CONCURRENTLY classes_teacher_idx
  ON app.classes (teacher_id);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.classes_teacher_idx;
