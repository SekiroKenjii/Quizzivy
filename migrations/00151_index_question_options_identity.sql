-- +goose NO TRANSACTION
-- +goose Up

-- The keys a matching item's composite FK references (00152): (question_id, id)
-- on the bank's options and (test_version_question_id, id) on the frozen ones,
-- so an item can only be keyed to an option of its own question.
--
-- Plain unique indexes, never attached as constraints. An FK may reference the
-- columns of a non-partial, non-deferrable unique index directly; the existing
-- question_options_ordinal_key cannot serve, being DEFERRABLE, and it covers
-- other columns anyway. Attaching one with ADD CONSTRAINT ... UNIQUE USING INDEX
-- would hand the index to the constraint, and dropping the constraint would drop
-- the index.
--
-- CONCURRENTLY, because both tables hold rows and are written to by every save
-- and every publish. A build that is interrupted leaves the index INVALID and
-- goose has not recorded the file, so each build is preceded by a drop: with
-- IF NOT EXISTS alone, the rerun would find the name taken and record the file
-- over an index that enforces nothing (00133 and 00138 do the same).
DROP INDEX CONCURRENTLY IF EXISTS app.question_options_question_id_id_key;
CREATE UNIQUE INDEX CONCURRENTLY question_options_question_id_id_key
  ON app.question_options (question_id, id);

DROP INDEX CONCURRENTLY IF EXISTS app.test_version_options_question_id_id_key;
CREATE UNIQUE INDEX CONCURRENTLY test_version_options_question_id_id_key
  ON app.test_version_options (test_version_question_id, id);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.test_version_options_question_id_id_key;
DROP INDEX CONCURRENTLY IF EXISTS app.question_options_question_id_id_key;
