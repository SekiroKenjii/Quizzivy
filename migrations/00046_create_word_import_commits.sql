-- +goose Up
CREATE TABLE app.word_import_commits (
  import_id uuid PRIMARY KEY REFERENCES app.word_imports(id) ON DELETE RESTRICT,
  request_id uuid NOT NULL,
  draft_revision bigint NOT NULL CHECK (draft_revision > 0),
  digest bytea NOT NULL CHECK (octet_length(digest) = 32),
  test_id uuid REFERENCES app.tests(id) ON DELETE SET NULL,
  committed_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  committed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX word_import_commits_test ON app.word_import_commits (test_id) WHERE test_id IS NOT NULL;
CREATE INDEX word_import_commits_actor ON app.word_import_commits (committed_by);

REVOKE UPDATE, DELETE ON app.word_import_commits FROM quizzivy_app;

-- +goose Down
DROP TABLE app.word_import_commits;
