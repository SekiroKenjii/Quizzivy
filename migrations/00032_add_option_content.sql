-- +goose Up
ALTER TABLE app.question_options
  ADD COLUMN content jsonb
    CHECK (content IS NULL OR (jsonb_typeof(content) = 'object'
      AND (content->>'format' = 'semantic_v1') IS TRUE));

ALTER TABLE app.test_version_options
  ADD COLUMN content jsonb
    CHECK (content IS NULL OR (jsonb_typeof(content) = 'object'
      AND (content->>'format' = 'semantic_v1') IS TRUE));

-- +goose Down
ALTER TABLE app.test_version_options DROP COLUMN content;
ALTER TABLE app.question_options DROP COLUMN content;
