-- +goose Up

-- What the app tells one user (T-R4.10a). The reader's console words a row
-- from kind and params; no sentence is stored. kind is a dotted key and not an
-- enum, so a release adds a kind without a migration. Names inside params are
-- copies taken when the row was written (deviation D-28).
CREATE TABLE app.notifications (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  params     jsonb NOT NULL DEFAULT '{}'::jsonb,
  -- NULL when the notification leads nowhere. A NULL makes the check NULL,
  -- which a CHECK accepts; the JSON value null is refused.
  target     jsonb,
  -- What the row is about for its user. A second notice with the same key
  -- merges into this row instead of adding one.
  dedupe_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  read_at    timestamptz,

  CONSTRAINT notifications_kind_check
    CHECK (char_length(kind) <= 64 AND kind ~ '^[a-z][a-z_]*(\.[a-z][a-z_]*)+$'),
  CONSTRAINT notifications_params_shape
    CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 4096),
  CONSTRAINT notifications_target_shape
    CHECK (jsonb_typeof(target) = 'object' AND octet_length(target::text) <= 1024),
  CONSTRAINT notifications_dedupe_key_check
    CHECK (char_length(dedupe_key) BETWEEN 1 AND 200),
  CONSTRAINT notifications_user_dedupe_key UNIQUE (user_id, dedupe_key)
);

-- The reader's list: one user's rows, newest first, by keyset on id. Its
-- leading column also serves the cascade from app.users.
CREATE INDEX notifications_user_recent_idx ON app.notifications (user_id, id DESC);
-- The unread count the shell polls, and "mark all read". It holds unread rows
-- only, so it stays small.
CREATE INDEX notifications_unread_idx ON app.notifications (user_id) WHERE read_at IS NULL;
-- The daily prune.
CREATE INDEX notifications_created_at_idx ON app.notifications (created_at);

CREATE TRIGGER notifications_set_updated_at BEFORE UPDATE ON app.notifications
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- +goose Down
DROP TABLE app.notifications;
