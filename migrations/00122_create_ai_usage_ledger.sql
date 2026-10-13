-- +goose Up

-- One row for every call the provider metered (T-D4.31, D21): who made it, for
-- which feature and tier, the four token counts the provider reported, the
-- credits charged and the provider's request id, so the month's invoice can be
-- reconciled with the pool. month is the period the credits were charged to,
-- which is the reservation's and not always the month the row was written in.
--
-- Append-only, as audit_log and attempt_events are: the app role may insert and
-- read and nothing else, so no handler can rewrite what was spent. user_id is
-- set to NULL when the person is deleted (the row must outlive its actor), which
-- the referential action does as the table's owner.
CREATE TABLE app.ai_usage_ledger (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id            uuid REFERENCES app.users (id) ON DELETE SET NULL,
  month              date NOT NULL,
  feature            text NOT NULL,
  tier               text NOT NULL,
  input_tokens       bigint NOT NULL,
  output_tokens      bigint NOT NULL,
  cache_read_tokens  bigint NOT NULL,
  cache_write_tokens bigint NOT NULL,
  credits            bigint NOT NULL,
  request_id         text,
  created_at         timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ai_usage_ledger_feature_check
    CHECK (feature ~ '^[a-z][a-z_]{0,39}$'),
  CONSTRAINT ai_usage_ledger_tier_check
    CHECK (tier IN ('fast', 'capable')),
  CONSTRAINT ai_usage_ledger_tokens_check
    CHECK (input_tokens >= 0 AND output_tokens >= 0 AND cache_read_tokens >= 0 AND cache_write_tokens >= 0),
  CONSTRAINT ai_usage_ledger_credits_check
    CHECK (credits >= 0),
  CONSTRAINT ai_usage_ledger_request_id_check
    CHECK (request_id IS NULL OR char_length(request_id) BETWEEN 1 AND 200)
);

CREATE INDEX ai_usage_ledger_user_month_idx ON app.ai_usage_ledger (user_id, month);
CREATE INDEX ai_usage_ledger_month_idx ON app.ai_usage_ledger (month);

-- 00009's default privileges give the app role full DML on every new table.
REVOKE UPDATE, DELETE ON app.ai_usage_ledger FROM quizzivy_app;

-- +goose Down
DROP TABLE app.ai_usage_ledger;
