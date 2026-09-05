-- +goose Up

-- G-05's "Ghi chú của bạn": the teacher's private note on one attempt, read
-- only through /admin. It is a column rather than a table because there is
-- one teacher and one note, and the audit log already keeps the history of
-- what was done about the attempt.
ALTER TABLE app.attempts ADD COLUMN teacher_note text
  CHECK (teacher_note IS NULL OR length(teacher_note) <= 2000);

-- +goose Down

ALTER TABLE app.attempts DROP COLUMN teacher_note;
