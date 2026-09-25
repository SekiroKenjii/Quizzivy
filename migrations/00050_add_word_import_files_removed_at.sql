-- +goose Up
ALTER TABLE app.word_imports
  ADD COLUMN files_removed_at timestamptz,
  ADD COLUMN closed_idle boolean NOT NULL DEFAULT false,
  ADD CONSTRAINT word_imports_files_removed_terminal
    CHECK (files_removed_at IS NULL OR status IN ('committed', 'cancelled')),
  ADD CONSTRAINT word_imports_closed_idle_cancelled
    CHECK (NOT closed_idle OR status = 'cancelled');
CREATE INDEX word_imports_retention ON app.word_imports (status, updated_at, id)
  WHERE files_removed_at IS NULL;

-- +goose Down
DROP INDEX app.word_imports_retention;
ALTER TABLE app.word_imports
  DROP CONSTRAINT word_imports_closed_idle_cancelled,
  DROP CONSTRAINT word_imports_files_removed_terminal,
  DROP COLUMN closed_idle,
  DROP COLUMN files_removed_at;
