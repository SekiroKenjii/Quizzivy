-- +goose Up
ALTER TABLE app.word_import_runs ADD CONSTRAINT word_import_runs_source_identity UNIQUE (id,import_id,source_revision);
ALTER TABLE app.word_import_source_set_items ADD CONSTRAINT word_import_source_items_identity UNIQUE (import_id,revision,role,source_id);

CREATE TABLE app.word_import_artifact_sets (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  import_id uuid NOT NULL,
  run_id uuid NOT NULL,
  source_revision bigint NOT NULL,
  source_id uuid NOT NULL,
  role text NOT NULL,
  claim_token bigint NOT NULL CHECK (claim_token > 0),
  stage text NOT NULL CHECK (stage IN ('normalization','extraction','recognition','validation')),
  component_version text NOT NULL CHECK (length(component_version) BETWEEN 1 AND 200),
  plan_digest bytea NOT NULL CHECK (octet_length(plan_digest)=32),
  manifest jsonb NOT NULL CHECK (jsonb_typeof(manifest)='object' AND octet_length(manifest::text)<=131072),
  file_count integer NOT NULL CHECK (file_count BETWEEN 1 AND 512),
  bytes bigint NOT NULL CHECK (bytes BETWEEN 1 AND 268435456),
  ready boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  completed_at timestamptz,
  CHECK (ready = (completed_at IS NOT NULL)),
  UNIQUE (run_id,claim_token,role,stage,component_version),
  UNIQUE (id,import_id),
  FOREIGN KEY (run_id,import_id,source_revision) REFERENCES app.word_import_runs(id,import_id,source_revision) ON DELETE RESTRICT,
  FOREIGN KEY (import_id,source_revision,role,source_id) REFERENCES app.word_import_source_set_items(import_id,revision,role,source_id) ON DELETE RESTRICT
);
CREATE INDEX word_import_artifact_sets_reuse ON app.word_import_artifact_sets(import_id,source_revision,role,stage,component_version,created_at DESC) WHERE ready;
CREATE INDEX word_import_artifact_sets_source ON app.word_import_artifact_sets(source_id);
CREATE INDEX word_import_artifact_sets_pending ON app.word_import_artifact_sets(created_at) WHERE NOT ready;

CREATE TABLE app.word_import_artifacts (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  set_id uuid NOT NULL,
  import_id uuid NOT NULL,
  ordinal integer NOT NULL CHECK (ordinal BETWEEN 0 AND 511),
  name text NOT NULL CHECK (name ~ '^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,119}$'),
  kind text NOT NULL CHECK (kind IN ('normalized_docx','source_pdf','source_page','source_blocks','source_image','candidate','validation')),
  content_type text NOT NULL CHECK (content_type IN ('application/json','application/pdf','image/png','application/vnd.openxmlformats-officedocument.wordprocessingml.document')),
  bytes bigint NOT NULL CHECK (bytes BETWEEN 1 AND 67108864),
  checksum_sha256 bytea NOT NULL CHECK (octet_length(checksum_sha256)=32),
  storage_key text NOT NULL UNIQUE,
  ready boolean NOT NULL DEFAULT false,
  UNIQUE (set_id,ordinal),
  UNIQUE (set_id,name),
  FOREIGN KEY (set_id,import_id) REFERENCES app.word_import_artifact_sets(id,import_id) ON DELETE RESTRICT
);
CREATE INDEX word_import_artifacts_import ON app.word_import_artifacts(import_id);
REVOKE UPDATE, DELETE ON app.word_import_artifact_sets, app.word_import_artifacts FROM quizzivy_app;
GRANT UPDATE (ready,completed_at) ON app.word_import_artifact_sets TO quizzivy_app;
GRANT UPDATE (ready) ON app.word_import_artifacts TO quizzivy_app;

-- +goose StatementBegin
CREATE FUNCTION app.protect_ready_import_artifact() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.ready THEN
    RAISE EXCEPTION 'completed import artifact is immutable';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER word_import_artifact_sets_immutable BEFORE UPDATE ON app.word_import_artifact_sets
  FOR EACH ROW EXECUTE FUNCTION app.protect_ready_import_artifact();
CREATE TRIGGER word_import_artifacts_immutable BEFORE UPDATE ON app.word_import_artifacts
  FOR EACH ROW EXECUTE FUNCTION app.protect_ready_import_artifact();

-- +goose StatementBegin
CREATE FUNCTION app.guard_import_artifact_insert() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  parent_ready boolean;
  expected_count integer;
BEGIN
  SELECT ready,file_count INTO parent_ready,expected_count
    FROM app.word_import_artifact_sets WHERE id=NEW.set_id AND import_id=NEW.import_id FOR UPDATE;
  IF parent_ready OR NEW.ordinal >= expected_count THEN
    RAISE EXCEPTION 'artifact set cannot accept this file';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER word_import_artifacts_insert BEFORE INSERT ON app.word_import_artifacts
  FOR EACH ROW EXECUTE FUNCTION app.guard_import_artifact_insert();

-- +goose StatementBegin
CREATE FUNCTION app.guard_import_artifact_completion() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
  actual_count integer;
  actual_bytes bigint;
  all_ready boolean;
BEGIN
  IF NEW.ready THEN
    SELECT count(*),coalesce(sum(bytes),0),bool_and(ready) INTO actual_count,actual_bytes,all_ready
      FROM app.word_import_artifacts WHERE set_id=NEW.id;
    IF actual_count <> NEW.file_count OR actual_bytes <> NEW.bytes OR all_ready IS DISTINCT FROM true THEN
      RAISE EXCEPTION 'artifact set is incomplete';
    END IF;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER word_import_artifact_sets_completion BEFORE INSERT OR UPDATE ON app.word_import_artifact_sets
  FOR EACH ROW EXECUTE FUNCTION app.guard_import_artifact_completion();

-- +goose Down
DROP TABLE app.word_import_artifacts;
DROP TABLE app.word_import_artifact_sets;
DROP FUNCTION app.protect_ready_import_artifact();
DROP FUNCTION app.guard_import_artifact_insert();
DROP FUNCTION app.guard_import_artifact_completion();
ALTER TABLE app.word_import_source_set_items DROP CONSTRAINT word_import_source_items_identity;
ALTER TABLE app.word_import_runs DROP CONSTRAINT word_import_runs_source_identity;
