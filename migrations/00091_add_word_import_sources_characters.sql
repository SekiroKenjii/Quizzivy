-- +goose Up
ALTER TABLE app.word_import_sources ADD COLUMN characters integer;
ALTER TABLE app.word_import_sources
  ADD CONSTRAINT word_import_sources_characters_range CHECK (characters BETWEEN 1 AND 100000),
  ADD CONSTRAINT word_import_sources_characters_format CHECK (
    (format = 'text' AND characters IS NOT NULL) OR (format <> 'text' AND characters IS NULL)
  );

-- +goose Down
ALTER TABLE app.word_import_sources DROP CONSTRAINT word_import_sources_characters_format;
ALTER TABLE app.word_import_sources DROP CONSTRAINT word_import_sources_characters_range;
ALTER TABLE app.word_import_sources DROP COLUMN characters;
