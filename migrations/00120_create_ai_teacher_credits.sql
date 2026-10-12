-- +goose Up

-- One teacher's allowance and use for one month (T-D4.31, D21). The allowance
-- is materialised when the row is created and is what the last constraint holds
-- spending to, as the pool's constraint does for the centre.
--
-- custom: the allowance was set for this teacher, so a change of the default
--   rule leaves it. paused: the teacher cannot use AI. Both belong to the
--   person, not the month, so a month's first row copies them from the
--   teacher's latest earlier row (ai_teacher_credits_user_idx serves that read).
-- granted_credits: what approved requests added this month. It is part of
--   allowance_credits and is not carried over, so a new month starts at the
--   teacher's own allowance.
-- warned_at: when the warning at warn_percent was raised, so it is raised once
--   a month.
CREATE TABLE app.ai_teacher_credits (
  month             date NOT NULL REFERENCES app.ai_credit_periods (month),
  user_id           uuid NOT NULL REFERENCES app.users (id) ON DELETE CASCADE,
  allowance_credits bigint NOT NULL,
  granted_credits   bigint NOT NULL DEFAULT 0,
  custom            boolean NOT NULL DEFAULT false,
  paused            boolean NOT NULL DEFAULT false,
  used_credits      bigint NOT NULL DEFAULT 0,
  reserved_credits  bigint NOT NULL DEFAULT 0,
  warned_at         timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (month, user_id),
  CONSTRAINT ai_teacher_credits_allowance_check
    CHECK (allowance_credits BETWEEN 1000 AND 20000000),
  CONSTRAINT ai_teacher_credits_counts_check
    CHECK (granted_credits >= 0 AND used_credits >= 0 AND reserved_credits >= 0),
  CONSTRAINT ai_teacher_credits_granted_check
    CHECK (granted_credits <= allowance_credits),
  CONSTRAINT ai_teacher_credits_within_allowance
    CHECK (used_credits + reserved_credits <= allowance_credits)
);

CREATE INDEX ai_teacher_credits_user_idx ON app.ai_teacher_credits (user_id, month DESC);

CREATE TRIGGER ai_teacher_credits_set_updated_at BEFORE UPDATE ON app.ai_teacher_credits
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- +goose Down
DROP TABLE app.ai_teacher_credits;
