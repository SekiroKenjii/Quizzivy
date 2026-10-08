-- +goose Up

-- The note a teacher leaves when publishing a version (T-R4.16a, DG-66). It is
-- nullable with no default, so the previous release's insert, which names
-- neither, keeps working during the rolling deploy; a version published
-- before this column, or without a note, reads NULL. The check is added NOT
-- VALID and then validated so the scan runs without blocking writes.
ALTER TABLE app.test_versions
  ADD COLUMN change_note text,
  ADD CONSTRAINT test_versions_change_note_check
    CHECK (char_length(change_note) BETWEEN 1 AND 200) NOT VALID;

ALTER TABLE app.test_versions VALIDATE CONSTRAINT test_versions_change_note_check;

-- +goose Down

ALTER TABLE app.test_versions
  DROP CONSTRAINT test_versions_change_note_check,
  DROP COLUMN change_note;
