-- +goose Up

CREATE TABLE app.attempt_answers (
  attempt_id      uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,

  -- An answer belongs to the frozen version, never to the mutable bank. This is
  -- the structural half of §7's versioning invariant.
  question_id     uuid NOT NULL
                    REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,
  payload         jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),

  -- [D-19] §7's pendingManual needs an indexable predicate and
  -- `final_score IS NULL` is not one: final_score is VIRTUAL and PG18 refuses
  -- to index it. Set at answer creation from the question type.
  requires_manual boolean NOT NULL DEFAULT false,

  auto_score      numeric(8,2) CHECK (auto_score   IS NULL OR auto_score   >= 0),
  manual_score    numeric(8,2) CHECK (manual_score IS NULL OR manual_score >= 0),
  final_score     numeric(8,2)
                    GENERATED ALWAYS AS (coalesce(manual_score, auto_score)) VIRTUAL,
  grader_comment  text,
  graded_by       uuid REFERENCES app.users(id) ON DELETE SET NULL,
  graded_at       timestamptz,
  updated_at      timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (attempt_id, question_id),
  CONSTRAINT attempt_answers_manual_grade_paired
    CHECK ((manual_score IS NULL) = (graded_at IS NULL))
);

CREATE INDEX attempt_answers_question_idx ON app.attempt_answers (question_id);
CREATE INDEX attempt_answers_pending_idx  ON app.attempt_answers (attempt_id)
  WHERE requires_manual AND manual_score IS NULL;

-- +goose StatementBegin
CREATE TRIGGER attempt_answers_set_updated_at BEFORE UPDATE ON app.attempt_answers
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();
-- +goose StatementEnd

-- +goose Down
DROP TABLE app.attempt_answers;
