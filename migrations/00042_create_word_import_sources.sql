-- +goose Up
CREATE TABLE app.word_imports (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  request_id uuid NOT NULL,
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200 AND title ~ '\S'),
  status text NOT NULL DEFAULT 'awaiting_sources'
    CHECK (status IN ('awaiting_sources','queued','processing','needs_review','committing','committed','failed','cancelled')),
  revision bigint NOT NULL DEFAULT 1 CHECK (revision > 0),
  source_revision bigint CHECK (source_revision > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (created_by, request_id)
);
CREATE INDEX word_imports_history ON app.word_imports (created_at DESC, id DESC);
CREATE INDEX word_imports_status_history ON app.word_imports (status, created_at DESC, id DESC);
CREATE INDEX word_imports_title_search ON app.word_imports USING gin (app.immutable_unaccent(lower(title)) gin_trgm_ops);
CREATE TRIGGER word_imports_updated_at BEFORE UPDATE ON app.word_imports
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

CREATE TABLE app.word_import_source_sets (
  import_id uuid NOT NULL REFERENCES app.word_imports(id) ON DELETE RESTRICT,
  revision bigint NOT NULL CHECK (revision > 0),
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (import_id, revision)
);
CREATE INDEX word_import_source_sets_actor ON app.word_import_source_sets (created_by);
ALTER TABLE app.word_imports ADD CONSTRAINT word_imports_current_sources
  FOREIGN KEY (id, source_revision) REFERENCES app.word_import_source_sets(import_id, revision) ON DELETE RESTRICT;

CREATE TABLE app.word_import_sources (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  import_id uuid NOT NULL REFERENCES app.word_imports(id) ON DELETE RESTRICT,
  upload_id uuid NOT NULL,
  expected_revision bigint NOT NULL CHECK (expected_revision > 0),
  role text NOT NULL CHECK (role IN ('exam','answer_key')),
  filename text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
  format text NOT NULL CHECK (format = 'docx'),
  bytes bigint NOT NULL CHECK (bytes BETWEEN 1 AND 26214400),
  checksum_sha256 bytea NOT NULL CHECK (octet_length(checksum_sha256) = 32),
  storage_key text NOT NULL UNIQUE,
  uploaded_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  ready boolean NOT NULL DEFAULT false,
  source_revision bigint,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (ready = (source_revision IS NOT NULL)),
  UNIQUE (import_id, upload_id),
  UNIQUE (id, import_id, role, ready),
  FOREIGN KEY (import_id, source_revision) REFERENCES app.word_import_source_sets(import_id, revision) ON DELETE RESTRICT
);
CREATE INDEX word_import_sources_actor ON app.word_import_sources (uploaded_by);
CREATE INDEX word_import_sources_revision ON app.word_import_sources (import_id, source_revision);
CREATE INDEX word_import_sources_pending ON app.word_import_sources (created_at, id) WHERE NOT ready;
CREATE INDEX word_import_sources_filename_search ON app.word_import_sources USING gin (app.immutable_unaccent(lower(filename)) gin_trgm_ops);

CREATE TABLE app.word_import_source_set_items (
  import_id uuid NOT NULL,
  revision bigint NOT NULL,
  role text NOT NULL CHECK (role IN ('exam','answer_key')),
  source_id uuid NOT NULL,
  ready boolean NOT NULL DEFAULT true CHECK (ready),
  PRIMARY KEY (import_id, revision, role),
  FOREIGN KEY (import_id, revision) REFERENCES app.word_import_source_sets(import_id, revision) ON DELETE RESTRICT,
  FOREIGN KEY (source_id, import_id, role, ready) REFERENCES app.word_import_sources(id, import_id, role, ready) ON DELETE RESTRICT
);
CREATE INDEX word_import_source_set_items_source ON app.word_import_source_set_items (source_id, import_id, role, ready);

REVOKE UPDATE, DELETE ON app.word_import_source_sets, app.word_import_source_set_items FROM quizzivy_app;
REVOKE UPDATE, DELETE ON app.word_import_sources FROM quizzivy_app;
GRANT UPDATE (ready, source_revision) ON app.word_import_sources TO quizzivy_app;

-- +goose Down
DROP TABLE app.word_import_source_set_items;
DROP TABLE app.word_import_sources;
ALTER TABLE app.word_imports DROP CONSTRAINT word_imports_current_sources;
DROP TABLE app.word_import_source_sets;
DROP TABLE app.word_imports;
