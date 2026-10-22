-- +goose Up

-- "Choose exactly N" (T-R6.1a): a multiple choice that takes exactly N picks and
-- is marked per correct pick. NULL is a plain multiple choice, all or nothing,
-- so every existing question keeps its rule. Seven is the 8-option maximum
-- (DG-63) less one. The frozen copy carries the rule, because marking reads the
-- version, never the bank.
--
-- Both tables hold rows, so each CHECK is added NOT VALID and then validated.
-- The file runs in one transaction, so the ADD COLUMN's ACCESS EXCLUSIVE lock
-- is held through VALIDATE; the new column is NULL on every row and the tables
-- hold a centre's bank, so the scan is brief. lock_timeout makes the file fail
-- at once, to be run again, rather than queue every writer behind it (as
-- 00103 does).
SET LOCAL lock_timeout = '5s';

ALTER TABLE app.questions ADD COLUMN select_count smallint;
ALTER TABLE app.questions
  ADD CONSTRAINT questions_select_count_check
    CHECK (select_count IS NULL OR (type = 'multiple_choice' AND select_count BETWEEN 2 AND 7))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_select_count_check;

ALTER TABLE app.test_version_questions ADD COLUMN select_count smallint;
ALTER TABLE app.test_version_questions
  ADD CONSTRAINT test_version_questions_select_count_check
    CHECK (select_count IS NULL OR (type = 'multiple_choice' AND select_count BETWEEN 2 AND 7))
    NOT VALID;
ALTER TABLE app.test_version_questions VALIDATE CONSTRAINT test_version_questions_select_count_check;

-- +goose Down

SET LOCAL lock_timeout = '5s';

-- Refuses while a question or a frozen version question has a select_count, as
-- 00145 refuses for the types: the rule cannot be rebuilt once the column goes,
-- and an older binary would mark the question all or nothing.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE select_count IS NOT NULL)
     OR EXISTS (SELECT 1 FROM app.test_version_questions WHERE select_count IS NOT NULL) THEN
    RAISE EXCEPTION 'a choose-N question exists; dropping select_count would lose its rule';
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE app.test_version_questions DROP COLUMN select_count;
ALTER TABLE app.questions DROP COLUMN select_count;
