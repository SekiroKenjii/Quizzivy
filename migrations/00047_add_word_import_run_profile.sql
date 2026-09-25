-- +goose Up
ALTER TABLE app.word_import_runs
  ADD COLUMN profile jsonb NOT NULL DEFAULT '{}'::jsonb
  CONSTRAINT word_import_runs_profile_shape CHECK (jsonb_typeof(profile) = 'object' AND octet_length(profile::text) <= 1024);

-- +goose Down
ALTER TABLE app.word_import_runs DROP COLUMN profile;
