-- +goose Up

-- Whether a student may pause this group recording once it is playing
-- (T-D4.13, DG-183). NOT NULL with a constant default, which PostgreSQL stores
-- without rewriting the table, so the previous release's insert, which names no
-- such column, keeps working during the rolling deploy and every recording
-- written before this column reads true. A recording always has its policy
-- (max_plays, allow_seek and show_transcript_after_submit are plain columns
-- here), so unlike a question's audio there is no absent case to model.
ALTER TABLE app.group_recordings
  ADD COLUMN allow_pause boolean NOT NULL DEFAULT true;

-- +goose Down

ALTER TABLE app.group_recordings
  DROP COLUMN allow_pause;
