-- +goose Up

-- T-R5.14: the rows of an import job that passed every rule, in file order. The
-- worker takes the pending ones, 25 to a transaction, and settles each: created
-- (user_id is the account) or skipped_exists (the address belongs to an
-- account, which an import never changes). A row whose address was already an
-- account when the job was stored is stored skipped_exists; a row that lost a
-- race to a sign-up is settled the same way by the worker.
--
-- ordinal counts the stored rows from 0, not the file's lines, so it is also the
-- position in the hand-out sheet. email keeps the address as the file spelled
-- it, trimmed, and the account takes the same.
--
-- secret_ciphertext holds the temporary password of a row created for a sheet,
-- sealed under SECRETS_KEY with the job id and ordinal as associated data. It is
-- written in the transaction that creates the account and erased in the one that
-- hands the sheet out, so a row has a secret only while it is created and the
-- sheet is waiting. user_id and class_id are set null when the account or the
-- class goes, so the row outlives them.
CREATE TABLE app.user_import_rows (
  job_id            uuid NOT NULL REFERENCES app.user_import_jobs(id) ON DELETE CASCADE,
  ordinal           smallint NOT NULL,
  full_name         text NOT NULL,
  email             text NOT NULL,
  role_id           uuid NOT NULL REFERENCES app.roles(id),
  class_id          uuid REFERENCES app.classes(id) ON DELETE SET NULL,
  outcome           text NOT NULL DEFAULT 'pending',
  user_id           uuid REFERENCES app.users(id) ON DELETE SET NULL,
  secret_ciphertext bytea,

  PRIMARY KEY (job_id, ordinal),
  CONSTRAINT user_import_rows_ordinal_check
    CHECK (ordinal BETWEEN 0 AND 1999),
  CONSTRAINT user_import_rows_full_name_check
    CHECK (char_length(full_name) BETWEEN 1 AND 200),
  CONSTRAINT user_import_rows_email_check
    CHECK (char_length(email) BETWEEN 3 AND 254),
  CONSTRAINT user_import_rows_outcome_check
    CHECK (outcome IN ('pending', 'created', 'skipped_exists')),
  CONSTRAINT user_import_rows_secret_only_when_created
    CHECK (secret_ciphertext IS NULL OR outcome = 'created')
);

-- The cascades from app.users and app.classes.
CREATE INDEX user_import_rows_user_idx ON app.user_import_rows (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX user_import_rows_class_idx ON app.user_import_rows (class_id) WHERE class_id IS NOT NULL;

-- +goose Down
DROP TABLE app.user_import_rows;
