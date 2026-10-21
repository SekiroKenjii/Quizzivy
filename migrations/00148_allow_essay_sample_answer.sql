-- +goose Up

-- An essay may carry a sample answer, as a short answer may (T-R6.1a): the
-- grader reads it beside the student's text. 00011's rule allowed short answers
-- only. The new rule is added first, so no moment has neither, and it names
-- 'essay', which 00145 committed.
SET LOCAL lock_timeout = '5s';

ALTER TABLE app.questions
  ADD CONSTRAINT questions_sample_answer_only_written
    CHECK (sample_answer IS NULL OR type IN ('short_answer', 'essay'))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_sample_answer_only_written;
ALTER TABLE app.questions DROP CONSTRAINT questions_sample_answer_only_short_answer;

-- +goose Down

SET LOCAL lock_timeout = '5s';

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE type::text = 'essay' AND sample_answer IS NOT NULL) THEN
    RAISE EXCEPTION 'an essay has a sample answer; the original rule would refuse it';
  END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE app.questions
  ADD CONSTRAINT questions_sample_answer_only_short_answer
    CHECK (sample_answer IS NULL OR type = 'short_answer')
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_sample_answer_only_short_answer;
ALTER TABLE app.questions DROP CONSTRAINT questions_sample_answer_only_written;
