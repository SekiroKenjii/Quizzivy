-- +goose Up

-- The AI assistant's rules, one row for the whole centre (T-D4.31, D21). The
-- row is reference data the assistant cannot run without, so the migration
-- writes it (D-21): a $200 pool at $0.001 a credit, a default allowance of
-- 20,000 credits a teacher, a warning at 80% and a request path for a teacher
-- who runs out. The pool has one rule, pause everyone (DG-196), so there is no
-- column for it. The singleton key makes a second row impossible.
CREATE TABLE app.ai_settings (
  singleton                 boolean PRIMARY KEY DEFAULT true,
  pool_credits              bigint NOT NULL,
  default_allowance_credits bigint NOT NULL,
  warn_percent              smallint NOT NULL,
  on_teacher_limit          text NOT NULL,
  updated_at                timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT ai_settings_singleton CHECK (singleton),
  CONSTRAINT ai_settings_pool_check
    CHECK (pool_credits BETWEEN 1 AND 100000000),
  CONSTRAINT ai_settings_default_allowance_check
    CHECK (default_allowance_credits BETWEEN 1000 AND 10000000),
  CONSTRAINT ai_settings_warn_percent_check
    CHECK (warn_percent BETWEEN 1 AND 99),
  CONSTRAINT ai_settings_on_teacher_limit_check
    CHECK (on_teacher_limit IN ('pause', 'request'))
);

CREATE TRIGGER ai_settings_set_updated_at BEFORE UPDATE ON app.ai_settings
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

INSERT INTO app.ai_settings (pool_credits, default_allowance_credits, warn_percent, on_teacher_limit)
VALUES (200000, 20000, 80, 'request');

-- +goose Down
DROP TABLE app.ai_settings;
