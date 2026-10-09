-- +goose Up

-- The note a teacher leaves for the students of an assignment, drawn on the
-- Test intro (T-R4.11, DG-71). Nullable with no default, so the previous
-- release's insert keeps working; an assignment made before this column, or
-- without a note, reads NULL. The command stores NULL for a note that is
-- blank once trimmed, so the check never meets an empty string.
ALTER TABLE app.assignments
  ADD COLUMN student_note text
    CONSTRAINT assignments_student_note_check
    CHECK (char_length(btrim(student_note)) BETWEEN 1 AND 500);

-- +goose Down

ALTER TABLE app.assignments
  DROP CONSTRAINT assignments_student_note_check,
  DROP COLUMN student_note;
