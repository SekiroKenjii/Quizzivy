-- +goose Up

-- T-R5.3 (D15): the one-time links by which an account sets a password from an
-- email. Only the SHA-256 of a 256-bit token is stored, so a read of the table
-- yields nothing a person could present; the raw token exists only as the
-- sealed secret of its outbox row until the message is sent.
--
-- A link is claimed by one statement that sets used_at, so two submits of one
-- link set the password once. A newer link for the account, a password set and
-- a disable set revoked_at on the links they replace, and the three states a
-- link can end in (used, revoked, expired) are read alike by the lookup.
-- created_at is written by the application with the same clock as expires_at,
-- so the 24-hour bound holds to the microsecond and is the database's own
-- last line against a longer-lived link.
--
-- created_by is provenance and is not enforced: an Admin or the account's own
-- self-service request, null for a link a job issued. It is SET NULL so a
-- deleted account's links outlive it as rows nobody can use; user_id cascades,
-- since a link without its account is meaningless.
CREATE TABLE app.set_password_tokens (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id    uuid NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
  token_hash bytea NOT NULL,
  purpose    text NOT NULL,
  created_by uuid REFERENCES app.users (id) ON DELETE SET NULL,
  expires_at timestamptz NOT NULL,
  used_at    timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT set_password_tokens_token_hash_key UNIQUE (token_hash),
  CONSTRAINT set_password_tokens_token_hash_check
    CHECK (length(token_hash) = 32),
  CONSTRAINT set_password_tokens_purpose_check
    CHECK (purpose IN ('invite', 'reset')),
  CONSTRAINT set_password_tokens_lifetime_check
    CHECK (expires_at <= created_at + interval '24 hours')
);

-- Every write that replaces or ends an account's links finds its unused ones by
-- the account, and the cascade from app.users reaches rows of every state, so
-- the index is not partial. created_by has none: only an account that issues
-- links has rows to update there, and no operation deletes one.
CREATE INDEX set_password_tokens_user_idx ON app.set_password_tokens (user_id);

-- +goose Down
DROP TABLE app.set_password_tokens;
