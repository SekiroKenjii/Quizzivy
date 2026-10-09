-- +goose Up

-- When a student's result is released, and whether it shows the class average
-- (T-R4.11, DG-65). Both columns carry a constant default, so adding them
-- rewrites nothing, and the previous release's insert, which names neither,
-- keeps working during the rolling deploy: it gets the rule it always had,
-- results at once and no average.
ALTER TABLE app.assignments
  ADD COLUMN review_release text NOT NULL DEFAULT 'on_submit'
    CONSTRAINT assignments_review_release_check
    CHECK (review_release IN ('on_submit', 'after_close')),
  ADD COLUMN review_show_class_average boolean NOT NULL DEFAULT false;

-- +goose Down

ALTER TABLE app.assignments
  DROP CONSTRAINT assignments_review_release_check,
  DROP COLUMN review_show_class_average,
  DROP COLUMN review_release;
