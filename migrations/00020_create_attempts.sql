-- +goose Up

-- The hot path: everything here is read on every autosave.
CREATE TABLE app.attempts (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  assignment_id     uuid NOT NULL REFERENCES app.assignments(id) ON DELETE RESTRICT,
  test_version_id   uuid NOT NULL REFERENCES app.test_versions(id) ON DELETE RESTRICT,
  student_id        uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  attempt_no        smallint NOT NULL CHECK (attempt_no > 0),
  status            app.attempt_status NOT NULL DEFAULT 'in_progress',
  session_id        uuid NOT NULL,

  -- [D-02] Drawn once at creation. The order is a pure function of
  -- (seed, question_id), so a reload cannot reshuffle and rebind stored answers
  -- to the wrong options -- silent corruption of the grade rather than a crash.
  shuffle_seed      bigint NOT NULL,

  -- [D-03] sendBeacon cannot set an Authorization header and a 15-minute access
  -- token is normally expired by the pagehide of a 60-minute test. Hashed for
  -- the same reason refresh tokens are.
  beacon_token_hash bytea NOT NULL CHECK (length(beacon_token_hash) = 32),

  started_at        timestamptz NOT NULL DEFAULT now(),
  deadline_at       timestamptz NOT NULL,
  submitted_at      timestamptz,
  graded_at         timestamptz,
  score_earned      numeric(8,2) CHECK (score_earned IS NULL OR score_earned >= 0),
  score_total       numeric(8,2) CHECK (score_total  IS NULL OR score_total  > 0),
  focus_loss_count  integer NOT NULL DEFAULT 0 CHECK (focus_loss_count >= 0),
  flagged           boolean NOT NULL DEFAULT false,
  void_reason       text,

  UNIQUE (assignment_id, student_id, attempt_no),
  CHECK (deadline_at > started_at),
  CONSTRAINT attempts_live_not_submitted
    CHECK (status <> 'in_progress' OR submitted_at IS NULL),
  CONSTRAINT attempts_graded_has_timestamp
    CHECK (status <> 'graded' OR graded_at IS NOT NULL),
  CONSTRAINT attempts_void_has_reason
    CHECK ((status = 'voided') = (void_reason IS NOT NULL))
);

-- What makes creating an attempt safely idempotent under a double-tap: the
-- second insert loses the race and the handler resumes instead.
CREATE UNIQUE INDEX attempts_one_live
  ON app.attempts (assignment_id, student_id) WHERE status = 'in_progress';

CREATE INDEX attempts_assignment_status_idx ON app.attempts (assignment_id, status);
CREATE INDEX attempts_student_started_idx   ON app.attempts (student_id, started_at DESC);
CREATE INDEX attempts_version_idx           ON app.attempts (test_version_id);

-- The two dashboard counts §8 needs. Both partial and therefore near-empty most
-- of the time.
CREATE INDEX attempts_grading_queue_idx
  ON app.attempts (submitted_at) WHERE status = 'submitted';
CREATE INDEX attempts_flagged_idx
  ON app.attempts (id) WHERE flagged;

-- +goose Down
DROP TABLE app.attempts;
