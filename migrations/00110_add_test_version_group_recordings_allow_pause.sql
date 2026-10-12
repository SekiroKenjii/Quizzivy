-- +goose Up

-- The pause rule frozen with a published group recording (T-D4.13, DG-183):
-- publish copies it from app.group_recordings. NOT NULL with a constant
-- default for the reasons of 00109; a version published before this column, or
-- by the previous binary, reads true.
ALTER TABLE app.test_version_group_recordings
  ADD COLUMN allow_pause boolean NOT NULL DEFAULT true;

-- +goose Down

ALTER TABLE app.test_version_group_recordings
  DROP COLUMN allow_pause;
