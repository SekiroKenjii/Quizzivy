-- +goose Up

CREATE TABLE app.users (
  id                   uuid PRIMARY KEY DEFAULT uuidv7(),
  -- text with a length CHECK rather than varchar(n): varchar gives no
  -- performance benefit and makes future changes harder. 254 is RFC 5321's
  -- address limit.
  email                text NOT NULL CHECK (length(email) BETWEEN 3 AND 254),
  full_name            text NOT NULL CHECK (length(full_name) BETWEEN 1 AND 200),
  role                 app.user_role NOT NULL DEFAULT 'student',
  password_hash        text,                  -- NULL = Google-only account (§5.1)
  must_change_password boolean NOT NULL DEFAULT false,
  disabled_at          timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),

  -- [D-16] §5.4: "Google-only users never hit /change-password." A constraint
  -- rather than a convention, because the redirect is global and a passwordless
  -- account flagged this way would be trapped on a page it cannot complete.
  CONSTRAINT users_must_change_needs_password
    CHECK (NOT must_change_password OR password_hash IS NOT NULL)
);

-- Case-insensitive uniqueness must be an expression index. A plain
-- UNIQUE (email) would let A@x.com and a@x.com coexist, which breaks §5.1's
-- rule that a verified Google email links to "the" existing user.
CREATE UNIQUE INDEX users_email_lower_key ON app.users (lower(email));

-- §8's active-student count. The table is tiny; this exists so the dashboard
-- query is not a repeated seq scan, not because it is hot.
CREATE INDEX users_role_active_idx ON app.users (role) WHERE disabled_at IS NULL;

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON app.users
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

CREATE TABLE app.user_identities (
  id               uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id          uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  -- text + CHECK rather than an enum: a second provider is a plausible v2
  -- change, and ALTER TYPE during a deploy is worse than ALTER TABLE.
  provider         text NOT NULL CHECK (provider IN ('google')),
  provider_user_id text NOT NULL,             -- Google `sub`, immutable
  email_at_link    text NOT NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),

  -- §5.3 step 4's "identity exists -> log in" lookup. Also stops one Google
  -- account reaching two users.
  UNIQUE (provider, provider_user_id),

  -- [D-08] §15's DELETE /auth/google/link and §7's linkedProviders both assume
  -- at most one Google identity per user; without this, "unlink" is ambiguous.
  -- This also makes a separate (user_id) index redundant.
  UNIQUE (user_id, provider)
);

-- +goose Down

DROP TABLE IF EXISTS app.user_identities;
DROP TABLE IF EXISTS app.users;
