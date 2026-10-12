-- +goose Up

-- The note a teacher leaves for the students of an assignment, drawn on the
-- Test intro (T-R4.11, DG-71). Nullable with no default, so the previous
-- release's insert keeps working; an assignment made before this column, or
-- without a note, reads NULL. The command stores NULL for a note that is
-- blank once trimmed, so the check never meets an empty string. Trimmed is
-- the whitespace JavaScript's trim() removes, the set the answered rule
-- (shared/answered) and the command's own trim use, not btrim's space alone.
ALTER TABLE app.assignments
  ADD COLUMN student_note text
    CONSTRAINT assignments_student_note_check
    CHECK (char_length(btrim(student_note, E'\u0009\u000A\u000B\u000C\u000D\u0020\u00A0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200A\u2028\u2029\u202F\u205F\u3000\uFEFF')) BETWEEN 1 AND 500);

-- +goose Down

ALTER TABLE app.assignments
  DROP CONSTRAINT assignments_student_note_check,
  DROP COLUMN student_note;
