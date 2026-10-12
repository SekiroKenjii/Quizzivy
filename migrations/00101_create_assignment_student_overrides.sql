-- +goose Up

-- What one teacher changed about one student's turn at an assignment (T-R4.12):
-- a later close, a longer time limit, more attempts, and the reason. A row
-- belongs to the assignment and to the student, so removing either removes it.
-- A missing row is the assignment as everyone has it. closes_at only ever
-- lengthens: the reader takes the later of it and the assignment's own close.
-- The check keeps a row from saying nothing; taking an override away is a
-- DELETE. reason is stored trimmed by the command, so its check reads the
-- length of what was written.
CREATE TABLE app.assignment_student_overrides (
  assignment_id    uuid NOT NULL REFERENCES app.assignments(id) ON DELETE CASCADE,
  student_id       uuid NOT NULL REFERENCES app.users(id) ON DELETE CASCADE,
  closes_at        timestamptz,
  duration_minutes integer,
  extra_attempts   smallint NOT NULL DEFAULT 0,
  reason           text NOT NULL,
  created_by       uuid REFERENCES app.users(id) ON DELETE SET NULL,
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),

  PRIMARY KEY (assignment_id, student_id),
  CONSTRAINT assignment_student_overrides_duration_check
    CHECK (duration_minutes BETWEEN 1 AND 600),
  CONSTRAINT assignment_student_overrides_extra_attempts_check
    CHECK (extra_attempts BETWEEN 0 AND 10),
  CONSTRAINT assignment_student_overrides_reason_check
    CHECK (char_length(reason) BETWEEN 1 AND 500),
  CONSTRAINT assignment_student_overrides_changes_check
    CHECK (closes_at IS NOT NULL OR duration_minutes IS NOT NULL OR extra_attempts > 0)
);

-- A student's overrides across assignments, and the cascade from app.users.
-- The primary key leads with assignment_id, so the cascade from
-- app.assignments needs nothing more.
CREATE INDEX assignment_student_overrides_student_idx
  ON app.assignment_student_overrides (student_id);

CREATE TRIGGER assignment_student_overrides_set_updated_at
  BEFORE UPDATE ON app.assignment_student_overrides
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- +goose Down
DROP TABLE app.assignment_student_overrides;
