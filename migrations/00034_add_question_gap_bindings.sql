-- +goose Up
ALTER TABLE app.question_blanks
  ADD COLUMN gap_id text CHECK (gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  ADD CONSTRAINT question_blanks_gap_key UNIQUE (question_id, gap_id);
ALTER TABLE app.test_version_blanks
  ADD COLUMN gap_id text CHECK (gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  ADD CONSTRAINT test_version_blanks_gap_key UNIQUE (test_version_question_id, gap_id);

ALTER TABLE app.questions DROP CONSTRAINT questions_check,
  ADD CONSTRAINT questions_check CHECK (prompt_content IS NULL OR
    (jsonb_typeof(prompt_content) = 'object' AND (prompt_content->>'format' = 'semantic_v1') IS TRUE));
ALTER TABLE app.test_version_questions DROP CONSTRAINT test_version_questions_check,
  ADD CONSTRAINT test_version_questions_check CHECK (prompt_content IS NULL OR
    (jsonb_typeof(prompt_content) = 'object' AND (prompt_content->>'format' = 'semantic_v1') IS TRUE));

-- +goose Down
ALTER TABLE app.questions DROP CONSTRAINT questions_check,
  ADD CONSTRAINT questions_check CHECK (prompt_content IS NULL OR
    (jsonb_typeof(prompt_content) = 'object' AND (prompt_content->>'format' = 'semantic_v1') IS TRUE AND type <> 'fill_blank'));
ALTER TABLE app.test_version_questions DROP CONSTRAINT test_version_questions_check,
  ADD CONSTRAINT test_version_questions_check CHECK (prompt_content IS NULL OR
    (jsonb_typeof(prompt_content) = 'object' AND (prompt_content->>'format' = 'semantic_v1') IS TRUE AND type <> 'fill_blank'));
ALTER TABLE app.test_version_blanks DROP COLUMN gap_id;
ALTER TABLE app.question_blanks DROP COLUMN gap_id;
