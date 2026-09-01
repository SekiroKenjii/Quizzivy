-- +goose Up

-- §10.1's telemetry. Append-only: a row is a claim about what happened at a
-- moment, and a log the application can rewrite is not evidence.
CREATE TABLE app.attempt_events (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  attempt_id  uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,

  -- [D-01] Part of the uniqueness key, not decoration. clientSeq lives in
  -- sessionStorage, which does not survive a tab close or a device change, so a
  -- resumed attempt restarts the counter at 1. Keyed on (attempt, client_seq)
  -- alone, every event of the resumed session would be a duplicate-key failure
  -- -- silently losing exactly the resume timeline the teacher needs.
  session_id  uuid NOT NULL,

  -- Not an enum. §10.1's list looks closed, but an append-only telemetry log
  -- reliably gains kinds, and each new one would be a migration plus a
  -- deploy-ordering problem for a value that is never joined on.
  kind        text NOT NULL CHECK (kind <> '' AND length(kind) <= 40),

  occurred_at timestamptz NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  -- NULL means server-written: `resume` and `session_takeover` are marked
  -- server-side in §10.1 and have no client counter to carry. Giving them a
  -- fake one is not free -- the client's own sequence may legitimately start at
  -- 0, so any constant the server picked would eventually collide with a real
  -- event and, under the DO NOTHING below, silently discard the student's.
  -- UNIQUE treats NULLs as distinct, so server rows coexist while client rows
  -- still deduplicate.
  client_seq  integer CHECK (client_seq >= 0),
  question_id uuid REFERENCES app.test_version_questions(id) ON DELETE SET NULL,
  meta        jsonb,

  UNIQUE (attempt_id, session_id, client_seq)
);

COMMENT ON COLUMN app.attempt_events.client_seq IS
  'NULL for server-written events, which carry no client sequence.';

CREATE INDEX attempt_events_timeline_idx
  ON app.attempt_events (attempt_id, occurred_at);
CREATE INDEX attempt_events_question_idx
  ON app.attempt_events (question_id) WHERE question_id IS NOT NULL;

-- Answers "was the tab I am superseding still open?" without scanning the
-- attempt's whole timeline. Partial because the server's own resume and
-- session_takeover rows are written on a session's behalf and are not evidence
-- that its tab is alive -- two reloads in a row would otherwise read the first
-- reload's `resume` as a live rival and cry takeover.
CREATE INDEX attempt_events_session_liveness_idx
  ON app.attempt_events (attempt_id, session_id, received_at DESC)
  WHERE kind NOT IN ('resume', 'session_takeover');

-- 00009 promised this. Same reasoning as audit_log: append-only is a privilege
-- here, not a convention the next handler can forget.
REVOKE UPDATE, DELETE ON app.attempt_events FROM quizzivy_app;

-- +goose Down
DROP TABLE app.attempt_events;
