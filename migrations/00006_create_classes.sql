-- +goose Up

CREATE TABLE app.classes (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  name              text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
  description       text,
  self_join_enabled boolean NOT NULL DEFAULT true,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

-- No index beyond the PK. A single-teacher practice has single-digit classes;
-- §8's list is a seq scan and that is the correct plan.

CREATE TRIGGER classes_set_updated_at BEFORE UPDATE ON app.classes
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- §6: a join code is a BEARER SECRET. Whoever holds it can enrol.
CREATE TABLE app.class_join_codes (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  class_id   uuid NOT NULL REFERENCES app.classes(id) ON DELETE CASCADE,

  -- Only the hash is stored (§13.3), so a database dump does not hand over
  -- class access. This is also the only lookup path: /join/preview is
  --   WHERE code_hash = $1 AND revoked_at IS NULL AND expires_at > now()
  code_hash  bytea NOT NULL UNIQUE CHECK (length(code_hash) = 32),
  -- Last 4 characters, for admin display after the one-time reveal.
  code_hint  text NOT NULL CHECK (length(code_hint) = 4),

  expires_at timestamptz NOT NULL,
  max_uses   integer CHECK (max_uses IS NULL OR max_uses > 0),
  uses_count integer NOT NULL DEFAULT 0 CHECK (uses_count >= 0),
  revoked_at timestamptz,

  -- ON DELETE RESTRICT: the record of who issued a bearer secret must not be
  -- erasable by deleting a user.
  created_by uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),

  -- [D-09]
  CONSTRAINT class_join_codes_expiry_after_creation CHECK (expires_at > created_at),
  -- Exhaustion is enforced HERE, not only in the handler. §6.5 treats the code
  -- as a bearer secret, and a uses_count > max_uses row is a silent policy
  -- failure -- exactly the kind that is discovered after it matters.
  CONSTRAINT class_join_codes_not_over_used CHECK (max_uses IS NULL OR uses_count <= max_uses)
);

-- One ACTIVE code per class (§6.1). Note what this does not say: an EXPIRED
-- code still satisfies revoked_at IS NULL and so still occupies the slot. That
-- matches §6.1 -- only rotation revokes -- and means rotation must revoke the
-- old code before inserting the new one, inside one transaction.
CREATE UNIQUE INDEX class_join_codes_one_active
  ON app.class_join_codes (class_id) WHERE revoked_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS app.class_join_codes;
DROP TABLE IF EXISTS app.classes;
