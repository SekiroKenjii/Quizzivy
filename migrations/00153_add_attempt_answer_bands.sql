-- +goose Up

-- An essay's band, stored beside the points it converts to (T-R6.1b, plan 76):
-- 3 to 9 in half steps, with manual_score = round(band * points / 9, 2), so
-- final_score, finishGrading and every total stay as they are. A band is a
-- grade, so it needs the manual score it was converted to. numeric(3,1) rounds
-- 6.25 to 6.3 before the CHECK runs, and the step rule then refuses it.
ALTER TABLE app.attempt_answers ADD COLUMN band numeric(3,1);
ALTER TABLE app.attempt_answers
  ADD CONSTRAINT attempt_answers_band_step
    CHECK (band IS NULL OR (band BETWEEN 3 AND 9 AND band * 2 = trunc(band * 2)))
    NOT VALID,
  ADD CONSTRAINT attempt_answers_band_needs_score
    CHECK (band IS NULL OR manual_score IS NOT NULL)
    NOT VALID;
ALTER TABLE app.attempt_answers VALIDATE CONSTRAINT attempt_answers_band_step;
ALTER TABLE app.attempt_answers VALIDATE CONSTRAINT attempt_answers_band_needs_score;

-- The band per rubric criterion of a graded essay. Not append-only: regrading
-- replaces the rows, as a regrade overwrites manual_score (last write wins).
-- The second FK is composite, so a grade can name only a criterion of the
-- question it grades; RESTRICT, so a frozen criterion with grades cannot go.
CREATE TABLE app.attempt_answer_criteria (
  attempt_id   uuid NOT NULL,
  question_id  uuid NOT NULL,
  criterion_id uuid NOT NULL,
  band         numeric(3,1) NOT NULL
                 CONSTRAINT attempt_answer_criteria_band_step
                 CHECK (band BETWEEN 3 AND 9 AND band * 2 = trunc(band * 2)),

  PRIMARY KEY (attempt_id, question_id, criterion_id),
  CONSTRAINT attempt_answer_criteria_answer_fk FOREIGN KEY (attempt_id, question_id)
    REFERENCES app.attempt_answers (attempt_id, question_id) ON DELETE CASCADE,
  CONSTRAINT attempt_answer_criteria_criterion_fk FOREIGN KEY (question_id, criterion_id)
    REFERENCES app.test_version_rubric_criteria (test_version_question_id, id) ON DELETE RESTRICT
);

-- Serves the RESTRICT check when a frozen criterion is deleted.
CREATE INDEX attempt_answer_criteria_criterion_idx
  ON app.attempt_answer_criteria (question_id, criterion_id);

-- +goose Down

-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.attempt_answers WHERE band IS NOT NULL)
     OR EXISTS (SELECT 1 FROM app.attempt_answer_criteria) THEN
    RAISE EXCEPTION 'graded bands exist; an older binary would show their points without them';
  END IF;
END
$$;
-- +goose StatementEnd
DROP TABLE app.attempt_answer_criteria;
ALTER TABLE app.attempt_answers DROP COLUMN band;
