-- +goose Up

-- A teacher's request for more credits in a month (T-D4.31, DG-200). Approving
-- one raises that month's allowance by amount_credits; declining changes
-- nothing but the status. At most one request per teacher per month is pending,
-- which the partial unique index makes a fact of the data.
--
-- decided_by is NULL when the console decided it, which the audit row says.
CREATE TABLE app.ai_credit_requests (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  user_id        uuid NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
  month          date NOT NULL,
  amount_credits bigint NOT NULL,
  reason         text NOT NULL,
  status         text NOT NULL DEFAULT 'pending',
  decided_by     uuid REFERENCES app.users (id) ON DELETE SET NULL,
  decided_at     timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ai_credit_requests_amount_check
    CHECK (amount_credits BETWEEN 1000 AND 100000),
  CONSTRAINT ai_credit_requests_reason_check
    CHECK (char_length(reason) BETWEEN 1 AND 500),
  CONSTRAINT ai_credit_requests_status_check
    CHECK (status IN ('pending', 'approved', 'declined')),
  CONSTRAINT ai_credit_requests_decision_check
    CHECK ((status = 'pending') = (decided_at IS NULL))
);

CREATE UNIQUE INDEX ai_credit_requests_one_pending_idx
  ON app.ai_credit_requests (user_id, month) WHERE status = 'pending';
CREATE INDEX ai_credit_requests_user_idx ON app.ai_credit_requests (user_id, month);
CREATE INDEX ai_credit_requests_pending_idx
  ON app.ai_credit_requests (month, created_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE app.ai_credit_requests;
