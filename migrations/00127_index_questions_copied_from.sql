-- +goose NO TRANSACTION
-- +goose Up

-- The referencing side of questions_copied_from_id_fkey. PostgreSQL does not
-- index a foreign key's column, and ON DELETE SET NULL runs
-- UPDATE ... WHERE copied_from_id = $1 whenever an origin is deleted for good,
-- which without an index scans app.questions. Partial, because almost every
-- question is not a copy: the index holds the copies, as tvq_source_idx
-- (00017) holds the frozen questions that still name a source, and the action's
-- equality implies IS NOT NULL, so the planner uses it (EXPLAIN of the action's
-- statement shows an Index Scan on this index). CONCURRENTLY, because the table
-- is populated and in use, and one index to the file, so an interrupted build
-- leaves at most this one behind.
CREATE INDEX CONCURRENTLY questions_copied_from_idx
  ON app.questions (copied_from_id)
  WHERE copied_from_id IS NOT NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.questions_copied_from_idx;
