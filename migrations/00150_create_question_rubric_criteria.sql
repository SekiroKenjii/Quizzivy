-- +goose Up

-- An essay's rubric (T-R6.1b, DG-61 interim): up to six criteria per question,
-- each a label the grader scores on a band. When a question has criteria, its
-- band is their mean rounded half up to 0.5; without them the grader sets the
-- band directly. The count of six is the bank's rule (T-R6.3), not a CHECK.
-- The foreign keys lock the tables they reference while the file runs, so
-- lock_timeout fails it at once rather than queue writers behind it.
SET LOCAL lock_timeout = '5s';

-- [D-13] The ordinal key is DEFERRABLE INITIALLY IMMEDIATE for the editor's
-- reorder, as question_options_ordinal_key is (00012).
CREATE TABLE app.question_rubric_criteria (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL CHECK (ordinal >= 0),
  label       text NOT NULL CHECK (length(label) BETWEEN 1 AND 80 AND btrim(label) <> ''),

  CONSTRAINT question_rubric_criteria_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

-- The frozen copy, written at publish and never changed: the app role may not
-- UPDATE it, as 00039 does for the frozen group graph. The second key is what
-- attempt_answer_criteria's composite FK references (00153), so a grade can
-- only name a criterion of the question it grades.
CREATE TABLE app.test_version_rubric_criteria (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 0),
  label                    text NOT NULL CHECK (length(label) BETWEEN 1 AND 80 AND btrim(label) <> ''),

  CONSTRAINT test_version_rubric_criteria_ordinal_key UNIQUE (test_version_question_id, ordinal),
  CONSTRAINT test_version_rubric_criteria_question_identity_key UNIQUE (test_version_question_id, id)
);

REVOKE UPDATE ON app.test_version_rubric_criteria FROM quizzivy_app;

-- +goose Down

SET LOCAL lock_timeout = '5s';

DROP TABLE app.test_version_rubric_criteria;
DROP TABLE app.question_rubric_criteria;
