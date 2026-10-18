-- +goose Up

-- T-R5.16 (D-050): the anonymised accounts whose profile photo objects the
-- daily housekeeping has removed. Anonymisation (core/maintenance, as the
-- maintenance role) clears users.avatar_key, but the role holds no storage
-- credential, so the objects under avatars/{user_id}/ stay until the
-- housekeeping, which runs in the API, deletes every object under that prefix
-- and then writes the account's row here. An anonymised account without a row
-- is one whose prefix has not been emptied yet, so each account is listed once,
-- and a deployment that gains a media bucket later still finds every account.
--
-- The application only reads and adds rows: a sweep is never undone, and the
-- row goes with the account.
CREATE TABLE app.avatar_sweeps (
  user_id  uuid PRIMARY KEY REFERENCES app.users(id) ON DELETE CASCADE,
  swept_at timestamptz NOT NULL
);

REVOKE UPDATE, DELETE ON app.avatar_sweeps FROM quizzivy_app;

-- +goose Down
DROP TABLE IF EXISTS app.avatar_sweeps;
