-- +goose Up

-- Where a member copy of a bank question came from (v0.11.7, the hotfix after
-- E1, docs/plan/74e-e1-group-move.md "After release"). Dragging a bank question
-- into a shared-context group copies it; the copy names the question it was
-- made from, so dragging it straight back out can restore the original instead
-- of leaving a second bank question with the same content.
--
-- Nullable with no default and no backfill: the previous release's insert names
-- no such column and keeps working through the rolling deploy, and a copy that
-- release wrote reads NULL and is never restored. ON DELETE SET NULL, the choice
-- of test_version_questions.source_question_id (D-07): the link is informational,
-- so deleting the origin must not be vetoed by a copy, and a copy whose origin
-- is gone is an ordinary member.
--
-- The REFERENCES stays inline in the ADD COLUMN. In one statement PostgreSQL
-- knows the new column is all NULL and skips the validation scan; a separate
-- ADD CONSTRAINT would scan the table under its lock (1M rows, 320 MB: 1.6 ms
-- against 6.7 s). The index is 00127.
ALTER TABLE app.questions
  ADD COLUMN copied_from_id uuid
    CONSTRAINT questions_copied_from_id_fkey
    REFERENCES app.questions (id) ON DELETE SET NULL;

-- +goose Down

ALTER TABLE app.questions
  DROP CONSTRAINT questions_copied_from_id_fkey,
  DROP COLUMN copied_from_id;
