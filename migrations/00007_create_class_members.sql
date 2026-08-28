-- +goose Up

CREATE TABLE app.class_members (
  class_id     uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,
  user_id      uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  joined_via   app.join_source NOT NULL,
  joined_at    timestamptz NOT NULL DEFAULT now(),

  -- [D-10] Who added them, when it was an admin.
  added_by     uuid REFERENCES app.users(id) ON DELETE SET NULL,

  -- [D-10] WHICH code they came through. §6.4 shows joined_via so the teacher
  -- can spot unexpected enrolments; recording the specific code means that
  -- after a rotation they can tell "joined via the code that leaked" from
  -- "joined via the current one".
  --
  -- RESTRICT, not SET NULL: codes are revoked, never deleted, and SET NULL
  -- would violate the consistency CHECK below.
  join_code_id uuid REFERENCES app.class_join_codes(id) ON DELETE RESTRICT,

  PRIMARY KEY (class_id, user_id),

  CONSTRAINT class_members_source_consistent
    CHECK ((joined_via = 'admin') = (join_code_id IS NULL))
);

-- Required, not optional: §9's /app/classes and assignment target resolution
-- both query by user_id, which the (class_id, user_id) PK cannot serve.
CREATE INDEX class_members_user_idx ON app.class_members (user_id);

-- +goose Down

DROP TABLE IF EXISTS app.class_members;
