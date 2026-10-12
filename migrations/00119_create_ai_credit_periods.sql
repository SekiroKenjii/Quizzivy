-- +goose Up

-- The centre's pool for one month (T-D4.31, D21). month is the first day of the
-- month in the centre's calendar zone; the row is created by the month's first
-- use, with pool_credits copied from ai_settings, so a later change of the plan
-- never rewrites a month that has begun. added_credits is what an Admin added
-- that month and does not carry over.
--
-- The hard stop is the last constraint: what is spent and what is held for calls
-- in flight never exceed the pool and what was added. Every path that moves a
-- credit does it in an UPDATE of this row, so the database, not the code,
-- refuses the call that would overspend.
CREATE TABLE app.ai_credit_periods (
  month            date PRIMARY KEY,
  pool_credits     bigint NOT NULL,
  added_credits    bigint NOT NULL DEFAULT 0,
  used_credits     bigint NOT NULL DEFAULT 0,
  reserved_credits bigint NOT NULL DEFAULT 0,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ai_credit_periods_month_check
    CHECK (month BETWEEN DATE '2026-01-01' AND DATE '2999-12-01' AND extract(day FROM month) = 1),
  CONSTRAINT ai_credit_periods_pool_check
    CHECK (pool_credits >= 1),
  CONSTRAINT ai_credit_periods_counts_check
    CHECK (added_credits >= 0 AND used_credits >= 0 AND reserved_credits >= 0),
  CONSTRAINT ai_credit_periods_within_pool
    CHECK (used_credits + reserved_credits <= pool_credits + added_credits)
);

CREATE TRIGGER ai_credit_periods_set_updated_at BEFORE UPDATE ON app.ai_credit_periods
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- +goose Down
DROP TABLE app.ai_credit_periods;
