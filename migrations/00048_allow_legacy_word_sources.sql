-- +goose Up
ALTER TABLE app.word_import_sources DROP CONSTRAINT word_import_sources_format_check;
ALTER TABLE app.word_import_sources
  ADD CONSTRAINT word_import_sources_format_check CHECK (format IN ('docx', 'doc'));

-- +goose Down
ALTER TABLE app.word_import_sources DROP CONSTRAINT word_import_sources_format_check;
ALTER TABLE app.word_import_sources
  ADD CONSTRAINT word_import_sources_format_check CHECK (format = 'docx') NOT VALID;
