-- +goose Up

-- The alt text a teacher writes for a question's image (T-R4.62a, DG-111). It
-- is nullable with no default and no backfill, so the previous release's
-- insert, which names no such column, keeps working during the rolling deploy;
-- a question written before this column reads NULL. The same 1 to 1000
-- characters as ContentImage.alt in the contract.
ALTER TABLE app.questions
  ADD COLUMN media_alt text
    CONSTRAINT questions_media_alt_check
    CHECK (char_length(media_alt) BETWEEN 1 AND 1000);

-- +goose Down

ALTER TABLE app.questions
  DROP CONSTRAINT questions_media_alt_check,
  DROP COLUMN media_alt;
