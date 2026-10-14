-- +goose Up

-- The centre's own settings, one row for the whole installation (T-R5.5): who
-- the centre is and how it is reached, its logos, how people sign in, and how
-- long it keeps answers, focus events and disabled accounts. The row is
-- reference data the front door and sign-in cannot run without, so the
-- migration writes it (D-21). The boolean key that only true satisfies makes a
-- second row impossible, and the app role may neither insert nor delete one.
CREATE TABLE app.organization_settings (
  id                              boolean PRIMARY KEY DEFAULT true,
  name                            text NOT NULL,
  short_name                      text,
  website                         text,
  contact_email                   text,
  front_desk_phone                text,
  front_desk_hours                text,
  address                         text,
  logo_sha256                     bytea,
  logo_type                       text,
  logo_dark_sha256                bytea,
  logo_dark_type                  text,
  google_sign_in                  boolean NOT NULL DEFAULT true,
  password_sign_in                boolean NOT NULL DEFAULT true,
  force_password_change           boolean NOT NULL DEFAULT true,
  join_codes_enabled              boolean NOT NULL DEFAULT true,
  allowed_google_domains          text[] NOT NULL DEFAULT '{}',
  keep_answers_months             smallint,
  keep_focus_events_months        smallint NOT NULL DEFAULT 13,
  anonymise_disabled_after_months smallint,
  revision                        integer NOT NULL DEFAULT 1,
  updated_by                      uuid REFERENCES app.users (id) ON DELETE SET NULL,
  updated_at                      timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT organization_settings_singleton CHECK (id),
  CONSTRAINT organization_settings_name_check
    CHECK (char_length(btrim(name)) BETWEEN 1 AND 120),
  CONSTRAINT organization_settings_short_name_check
    CHECK (char_length(btrim(short_name)) BETWEEN 1 AND 40),
  CONSTRAINT organization_settings_website_check
    CHECK (char_length(website) BETWEEN 1 AND 200),
  CONSTRAINT organization_settings_contact_email_check
    CHECK (char_length(contact_email) BETWEEN 3 AND 254),
  CONSTRAINT organization_settings_front_desk_phone_check
    CHECK (front_desk_phone ~ '^[0-9+ ().-]{6,30}$'),
  CONSTRAINT organization_settings_front_desk_hours_check
    CHECK (char_length(front_desk_hours) BETWEEN 1 AND 120),
  CONSTRAINT organization_settings_address_check
    CHECK (char_length(address) BETWEEN 1 AND 300),
  CONSTRAINT organization_settings_logo_sha256_check
    CHECK (octet_length(logo_sha256) = 32),
  CONSTRAINT organization_settings_logo_dark_sha256_check
    CHECK (octet_length(logo_dark_sha256) = 32),
  CONSTRAINT organization_settings_logo_type_check
    CHECK (logo_type IN ('image/png', 'image/svg+xml')),
  CONSTRAINT organization_settings_logo_dark_type_check
    CHECK (logo_dark_type IN ('image/png', 'image/svg+xml')),
  CONSTRAINT organization_settings_logo_pair
    CHECK ((logo_sha256 IS NULL) = (logo_type IS NULL)),
  CONSTRAINT organization_settings_logo_dark_pair
    CHECK ((logo_dark_sha256 IS NULL) = (logo_dark_type IS NULL)),
  CONSTRAINT organization_settings_a_sign_in_method
    CHECK (google_sign_in OR password_sign_in),
  CONSTRAINT organization_settings_allowed_google_domains_check
    CHECK (
      cardinality(allowed_google_domains) <= 50
      AND array_position(allowed_google_domains, NULL) IS NULL
      AND array_to_string(allowed_google_domains, ' ') = lower(array_to_string(allowed_google_domains, ' '))
    ),
  CONSTRAINT organization_settings_keep_answers_months_check
    CHECK (keep_answers_months IN (12, 24)),
  CONSTRAINT organization_settings_keep_focus_events_months_check
    CHECK (keep_focus_events_months IN (6, 12, 13, 24)),
  CONSTRAINT organization_settings_anonymise_disabled_after_months_check
    CHECK (anonymise_disabled_after_months IN (12, 24)),
  CONSTRAINT organization_settings_revision_check CHECK (revision >= 1)
);

CREATE TRIGGER organization_settings_set_updated_at BEFORE UPDATE ON app.organization_settings
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

INSERT INTO app.organization_settings (name) VALUES ('Quizzivy');

REVOKE INSERT, DELETE ON app.organization_settings FROM quizzivy_app;

-- +goose Down
DROP TABLE app.organization_settings;
