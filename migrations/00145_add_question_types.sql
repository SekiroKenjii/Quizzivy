-- +goose Up

-- R6's three new question types (T-R6.1a). tfng is True / False / Not given,
-- three fixed options answered like a single choice; matching pairs items with
-- the question's options (T-R6.1b); essay is a written answer scored on a band.
--
-- A value added to an enum inside a transaction cannot be used until that
-- transaction commits, so nothing in this file names one: every CHECK that does
-- is in a later file. IF NOT EXISTS, because the Down below leaves the values in
-- place, and the up that follows it must not fail on finding them.
ALTER TYPE app.question_type ADD VALUE IF NOT EXISTS 'tfng';
ALTER TYPE app.question_type ADD VALUE IF NOT EXISTS 'matching';
ALTER TYPE app.question_type ADD VALUE IF NOT EXISTS 'essay';

-- +goose Down

-- PostgreSQL cannot drop an enum value, and rebuilding the type does not work
-- here: ALTER COLUMN ... TYPE re-parses every CHECK that compares `type`
-- (questions_sample_answer_only_short_answer among them), and each fails with
-- "operator does not exist". So the Down refuses while a row uses a new value,
-- and otherwise leaves the three values in place, unused and harmless. Resetting
-- the whole schema still removes them with the type (00002). See D-62 in
-- docs/plan/20-data-model.md.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE type::text IN ('tfng', 'matching', 'essay'))
     OR EXISTS (SELECT 1 FROM app.test_version_questions WHERE type::text IN ('tfng', 'matching', 'essay')) THEN
    RAISE EXCEPTION 'questions of type tfng, matching or essay exist; an older binary cannot read them';
  END IF;
END
$$;
-- +goose StatementEnd
