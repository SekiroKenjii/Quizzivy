-- +goose Up

-- What the app has to email (T-R5.2). A message is written in the transaction
-- that decided to send it, so a rolled-back decision sends nothing, and the
-- API and the worker each drain the table. Only the template name and its
-- parameters are stored; the words are rendered when the row is sent.
--
-- template is a checked slug and not an enum, as notifications.kind is (D-28):
-- the list of templates is Go's, so a release adds one without a migration.
-- secret_ciphertext is a set-password token sealed under SECRETS_KEY with the
-- row id as associated data; it is cleared in the statement that marks the
-- row sent or failed.
CREATE TABLE app.email_outbox (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  to_user_id        uuid REFERENCES app.users(id) ON DELETE SET NULL,
  to_address        text NOT NULL,
  template          text NOT NULL,
  locale            text NOT NULL,
  params            jsonb NOT NULL DEFAULT '{}'::jsonb,
  secret_ciphertext bytea,
  dedupe_key        text,
  deliver_after     timestamptz NOT NULL DEFAULT now(),
  attempts          smallint NOT NULL DEFAULT 0,
  last_error_code   text,
  provider_id       text,
  sent_at           timestamptz,
  failed_at         timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT email_outbox_to_address_length
    CHECK (char_length(to_address) BETWEEN 3 AND 254),
  CONSTRAINT email_outbox_template_check
    CHECK (template ~ '^[a-z_]{1,64}$'),
  CONSTRAINT email_outbox_locale_check
    CHECK (locale IN ('vi', 'en')),
  CONSTRAINT email_outbox_params_shape
    CHECK (jsonb_typeof(params) = 'object' AND octet_length(params::text) <= 4096),
  CONSTRAINT email_outbox_dedupe_key_check
    CHECK (char_length(dedupe_key) BETWEEN 1 AND 200),
  CONSTRAINT email_outbox_dedupe_key UNIQUE (dedupe_key),
  CONSTRAINT email_outbox_attempts_check
    CHECK (attempts BETWEEN 0 AND 10),
  CONSTRAINT email_outbox_last_error_code_check
    CHECK (char_length(last_error_code) <= 64),
  CONSTRAINT email_outbox_provider_id_check
    CHECK (char_length(provider_id) <= 200),
  CONSTRAINT email_outbox_one_outcome
    CHECK (sent_at IS NULL OR failed_at IS NULL)
);

-- The dispatchers' claim and the earliest pending row their timer waits for.
-- It holds pending rows only, so it stays as small as the queue.
CREATE INDEX email_outbox_pending_idx ON app.email_outbox (deliver_after)
  WHERE sent_at IS NULL AND failed_at IS NULL;
-- The day's budget: every batch counts the rows sent since local midnight.
CREATE INDEX email_outbox_sent_idx ON app.email_outbox (sent_at)
  WHERE sent_at IS NOT NULL;
-- The cascade from app.users sets to_user_id NULL by equality.
CREATE INDEX email_outbox_to_user_idx ON app.email_outbox (to_user_id)
  WHERE to_user_id IS NOT NULL;

-- +goose Down
DROP TABLE app.email_outbox;
