-- +goose Up

-- §13.4. Required entries: class enrolment (§6.5), join-code generation and
-- rotation, attempt reset/void/extend, password reset, media deletion, test
-- publish.
CREATE TABLE app.audit_log (
  -- bigint IDENTITY rather than uuid, for the reason §13.3 gives for
  -- attempt_events: an append-only log read by a narrow key is better served by
  -- a narrow sequential key. GENERATED ALWAYS AS IDENTITY over serial, per the
  -- Neon schema-design reference.
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

  -- SET NULL: an audit row must survive its actor. An audit trail that
  -- disappears when you delete the person is not an audit trail.
  actor_user_id uuid REFERENCES app.users(id) ON DELETE SET NULL,

  action        text NOT NULL CHECK (action <> ''),
  entity        text NOT NULL CHECK (entity <> ''),
  entity_id     uuid,
  occurred_at   timestamptz NOT NULL DEFAULT now(),
  ip            inet,
  user_agent    text,
  diff          jsonb
);

CREATE INDEX audit_log_entity_idx
  ON app.audit_log (entity, entity_id, occurred_at DESC);
CREATE INDEX audit_log_actor_idx
  ON app.audit_log (actor_user_id, occurred_at DESC);

-- No (occurred_at DESC) index yet. §8 has no global audit screen, and §13.6
-- says to leave B-tree skip scan passive rather than adding indexes
-- preemptively. Add it with the screen that needs it.

-- +goose Down

DROP TABLE IF EXISTS app.audit_log;
