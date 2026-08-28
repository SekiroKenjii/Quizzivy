-- +goose Up

-- §13.5 specifies these columns in prose only.
CREATE TABLE app.refresh_tokens (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id     uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,

  -- The rotation chain. §5.2's reuse detection revokes an entire family at
  -- once, which is why family_id is indexed and not merely stored.
  family_id   uuid NOT NULL,

  -- SHA-256 is exactly 32 bytes, so a wrong-length value is a bug caught at
  -- write time rather than a token that can never match. Lookup is by hash;
  -- the comparison is constant-time in Go (§13.5).
  token_hash  bytea NOT NULL UNIQUE CHECK (length(token_hash) = 32),

  issued_at   timestamptz NOT NULL DEFAULT now(),
  expires_at  timestamptz NOT NULL,
  revoked_at  timestamptz,

  -- Rows are pruned by expiry, so a pruned predecessor must not cascade-delete
  -- a live successor.
  replaced_by uuid REFERENCES app.refresh_tokens(id) ON DELETE SET NULL,

  user_agent  text,
  -- [D-10] inet, not text: it validates, normalises IPv6, and supports subnet
  -- containment if an abuse investigation ever needs it.
  ip          inet,

  CHECK (expires_at > issued_at)
);

CREATE INDEX refresh_tokens_family_idx ON app.refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_live_idx
  ON app.refresh_tokens (user_id) WHERE revoked_at IS NULL;
-- The cleanup job's index. Partial on revoked_at IS NULL because revoked rows
-- are deleted by family, not by expiry.
CREATE INDEX refresh_tokens_expiry_idx
  ON app.refresh_tokens (expires_at) WHERE revoked_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS app.refresh_tokens;
