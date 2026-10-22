-- +goose Up

-- A group member's options may keep their authored order ('fixed') on the two
-- new types that have options (T-R6.1a, QC-R6M-3). tfng's three options are
-- never shuffled, and matching's choices keep their order because authors refer
-- to their keys, so storing 'shuffle' for either would record an order the
-- engine never applies. 00035's rule allowed 'fixed' on the three old choice
-- types only. An essay has no options and stays out. The rule keeps its name;
-- dropping and adding it in one transaction leaves no moment without one.
SET LOCAL lock_timeout = '5s';

ALTER TABLE app.questions
  DROP CONSTRAINT questions_context_option_order,
  ADD CONSTRAINT questions_context_option_order
    CHECK (context_option_order IS DISTINCT FROM 'fixed'
           OR type IN ('single_choice', 'multiple_choice', 'true_false', 'tfng', 'matching'))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_context_option_order;

-- +goose Down

SET LOCAL lock_timeout = '5s';

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions
              WHERE context_option_order = 'fixed' AND type::text IN ('tfng', 'matching')) THEN
    RAISE EXCEPTION 'a tfng or matching group member keeps a fixed order; the original rule would refuse it';
  END IF;
END
$$;
-- +goose StatementEnd
ALTER TABLE app.questions
  DROP CONSTRAINT questions_context_option_order,
  ADD CONSTRAINT questions_context_option_order
    CHECK (context_option_order IS DISTINCT FROM 'fixed'
           OR type IN ('single_choice', 'multiple_choice', 'true_false'))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_context_option_order;
