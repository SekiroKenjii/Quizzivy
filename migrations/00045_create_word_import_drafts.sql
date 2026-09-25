-- +goose Up
CREATE TABLE app.word_import_drafts (
  import_id uuid PRIMARY KEY REFERENCES app.word_imports(id) ON DELETE RESTRICT,
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  run_id uuid NOT NULL,
  body jsonb NOT NULL CHECK (jsonb_typeof(body) = 'object' AND octet_length(body::text) <= 8388608),
  candidate_run_id uuid,
  candidate jsonb CHECK (jsonb_typeof(candidate) = 'object' AND octet_length(candidate::text) <= 8388608),
  edited_by uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK ((candidate IS NULL) = (candidate_run_id IS NULL)),
  FOREIGN KEY (run_id, import_id) REFERENCES app.word_import_runs(id, import_id) ON DELETE RESTRICT,
  FOREIGN KEY (candidate_run_id, import_id) REFERENCES app.word_import_runs(id, import_id) ON DELETE RESTRICT
);
CREATE INDEX word_import_drafts_run ON app.word_import_drafts (run_id, import_id);
CREATE INDEX word_import_drafts_candidate_run ON app.word_import_drafts (candidate_run_id, import_id) WHERE candidate_run_id IS NOT NULL;
CREATE INDEX word_import_drafts_editor ON app.word_import_drafts (edited_by) WHERE edited_by IS NOT NULL;
CREATE TRIGGER word_import_drafts_updated_at BEFORE UPDATE ON app.word_import_drafts
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

REVOKE DELETE ON app.word_import_drafts FROM quizzivy_app;

-- +goose Down
DROP TABLE app.word_import_drafts;
