-- +goose Up
CREATE TABLE app.word_import_runs (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  import_id uuid NOT NULL REFERENCES app.word_imports(id) ON DELETE RESTRICT,
  source_revision bigint NOT NULL,
  request_id uuid NOT NULL,
  requested_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  expected_revision bigint NOT NULL CHECK (expected_revision > 0),
  pipeline_version text NOT NULL CHECK (length(pipeline_version) BETWEEN 1 AND 100),
  status text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','succeeded','failed','cancelled')),
  stage text NOT NULL DEFAULT 'queued' CHECK (stage IN ('queued','source_validation','normalization','extraction','recognition','validation','ready')),
  attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  max_attempts integer NOT NULL DEFAULT 3 CHECK (max_attempts BETWEEN 1 AND 10),
  claim_token bigint NOT NULL DEFAULT 0 CHECK (claim_token >= 0),
  worker_id uuid,
  lease_until timestamptz,
  available_at timestamptz NOT NULL DEFAULT now(),
  error_code text CHECK (error_code ~ '^[A-Z][A-Z0-9_]{0,99}$'),
  result jsonb CHECK (jsonb_typeof(result) = 'object' AND octet_length(result::text) <= 8388608),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  CHECK (attempt_count <= max_attempts),
  CHECK ((status = 'running') = (lease_until IS NOT NULL AND worker_id IS NOT NULL)),
  CHECK (status = 'running' OR (lease_until IS NULL AND worker_id IS NULL)),
  CHECK ((status = 'succeeded') = (result IS NOT NULL)),
  CHECK ((status = 'succeeded') = (stage = 'ready')),
  CHECK ((status IN ('succeeded','failed','cancelled')) = (completed_at IS NOT NULL)),
  UNIQUE (import_id, request_id),
  UNIQUE (id, import_id),
  FOREIGN KEY (import_id,source_revision) REFERENCES app.word_import_source_sets(import_id,revision) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX word_import_runs_one_active ON app.word_import_runs(import_id) WHERE status IN ('queued','running');
CREATE INDEX word_import_runs_sources ON app.word_import_runs(import_id,source_revision);
CREATE INDEX word_import_runs_actor ON app.word_import_runs(requested_by);
CREATE INDEX word_import_runs_queue ON app.word_import_runs(available_at,created_at,id) WHERE status='queued';
CREATE INDEX word_import_runs_expiry ON app.word_import_runs(lease_until,id) WHERE status='running';
CREATE TRIGGER word_import_runs_updated_at BEFORE UPDATE ON app.word_import_runs
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

REVOKE UPDATE, DELETE ON app.word_import_runs FROM quizzivy_app;
GRANT UPDATE (status,stage,attempt_count,claim_token,worker_id,lease_until,available_at,error_code,result,completed_at)
  ON app.word_import_runs TO quizzivy_app;

-- +goose StatementBegin
CREATE FUNCTION app.protect_terminal_import_run() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('succeeded','failed','cancelled') THEN
    RAISE EXCEPTION 'terminal import run is immutable';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER word_import_runs_terminal BEFORE UPDATE ON app.word_import_runs
  FOR EACH ROW EXECUTE FUNCTION app.protect_terminal_import_run();

CREATE TABLE app.word_import_run_events (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  run_id uuid NOT NULL,
  import_id uuid NOT NULL,
  kind text NOT NULL CHECK (kind IN ('queued','claimed','stage','failed','retry_scheduled','lease_expired','cancelled','completed')),
  stage text NOT NULL,
  claim_token bigint NOT NULL CHECK (claim_token >= 0),
  attempt_count integer NOT NULL CHECK (attempt_count >= 0),
  worker_id uuid,
  actor_id uuid REFERENCES app.users(id) ON DELETE RESTRICT,
  error_code text CHECK (error_code ~ '^[A-Z][A-Z0-9_]{0,99}$'),
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  FOREIGN KEY (run_id,import_id) REFERENCES app.word_import_runs(id,import_id) ON DELETE RESTRICT
);
CREATE INDEX word_import_run_events_run ON app.word_import_run_events(run_id,import_id,id);
CREATE INDEX word_import_run_events_actor ON app.word_import_run_events(actor_id) WHERE actor_id IS NOT NULL;
REVOKE UPDATE, DELETE ON app.word_import_run_events FROM quizzivy_app;

-- +goose Down
DROP TABLE app.word_import_run_events;
DROP TABLE app.word_import_runs;
DROP FUNCTION app.protect_terminal_import_run();
