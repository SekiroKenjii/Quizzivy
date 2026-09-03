-- +goose Up

-- An attempt that ran out of time still has to be graded.
--
-- 00020 built this index WHERE status = 'submitted', which was right while
-- nothing could produce any other unfinished-but-closed state. T-3.9 adds one:
-- a deadline passing turns an attempt into 'timed_out', and it is graded
-- exactly the same way -- including its short_answer questions, which a teacher
-- still has to read.
--
-- Left as it was, a student who ran out of time on an essay question would
-- simply never appear in the teacher's grading queue, and nothing anywhere
-- would say so.
DROP INDEX app.attempts_grading_queue_idx;

CREATE INDEX attempts_grading_queue_idx
  ON app.attempts (submitted_at) WHERE status IN ('submitted', 'timed_out');

-- +goose Down

DROP INDEX app.attempts_grading_queue_idx;

CREATE INDEX attempts_grading_queue_idx
  ON app.attempts (submitted_at) WHERE status = 'submitted';
