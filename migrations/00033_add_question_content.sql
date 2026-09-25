-- +goose Up
ALTER TABLE app.questions
  ADD COLUMN prompt_content jsonb
    CHECK (prompt_content IS NULL OR (jsonb_typeof(prompt_content) = 'object'
      AND (prompt_content->>'format' = 'semantic_v1') IS TRUE
      AND type <> 'fill_blank')),
  ADD COLUMN explanation_content jsonb
    CHECK (explanation_content IS NULL OR (jsonb_typeof(explanation_content) = 'object'
      AND (explanation_content->>'format' = 'semantic_v1') IS TRUE));

ALTER TABLE app.test_version_questions
  ADD COLUMN prompt_content jsonb
    CHECK (prompt_content IS NULL OR (jsonb_typeof(prompt_content) = 'object'
      AND (prompt_content->>'format' = 'semantic_v1') IS TRUE
      AND type <> 'fill_blank')),
  ADD COLUMN explanation_content jsonb
    CHECK (explanation_content IS NULL OR (jsonb_typeof(explanation_content) = 'object'
      AND (explanation_content->>'format' = 'semantic_v1') IS TRUE));

-- +goose Down
ALTER TABLE app.test_version_questions DROP COLUMN prompt_content, DROP COLUMN explanation_content;
ALTER TABLE app.questions DROP COLUMN prompt_content, DROP COLUMN explanation_content;
