-- +goose Up

-- Whether the teacher asked for the AI assistant to read this import's files
-- (T-D4.33). The default is a constant, so a binary from before this column
-- keeps inserting imports and they read as false.
ALTER TABLE app.word_imports
  ADD COLUMN read_with_ai boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE app.word_imports
  DROP COLUMN read_with_ai;
