-- +goose Up

-- The alt text frozen with a published question (T-R4.62a, DG-111): publish
-- copies it from app.questions, so a later edit of the bank question never
-- reaches a version. Nullable, no default, no backfill: a version published
-- before this column reads NULL.
ALTER TABLE app.test_version_questions
  ADD COLUMN media_alt text
    CONSTRAINT test_version_questions_media_alt_check
    CHECK (char_length(media_alt) BETWEEN 1 AND 1000);

-- +goose Down

ALTER TABLE app.test_version_questions
  DROP CONSTRAINT test_version_questions_media_alt_check,
  DROP COLUMN media_alt;
