-- +goose Up

-- [§11.4] The count lives here rather than in the client because the obvious
-- client-side counter resets on reload, which makes maxPlays mean nothing.
--
-- Nothing enforces the limit. Over-limit plays are reported to the teacher, and
-- neither a play nor a submit is ever refused because of this number: blocking
-- on a round trip would punish bad wifi far more often than it would catch
-- anyone, and a student determined to replay can go offline anyway -- which
-- leaves a gap in the event log, which is what the timeline is for.
CREATE TABLE app.attempt_audio_plays (
  attempt_id     uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,

  -- RESTRICT, not CASCADE: a frozen version's questions are never deleted while
  -- an attempt references them, and a purge that would orphan this count should
  -- stop rather than silently drop evidence about listening behaviour.
  question_id    uuid NOT NULL
                   REFERENCES app.test_version_questions(id) ON DELETE RESTRICT,

  plays          integer NOT NULL DEFAULT 0 CHECK (plays >= 0),
  last_played_at timestamptz,

  PRIMARY KEY (attempt_id, question_id)
);

-- §8's "audio replays" summary reads by question across attempts; the primary
-- key leads with attempt_id and cannot serve it.
CREATE INDEX attempt_audio_plays_question_idx
  ON app.attempt_audio_plays (question_id);

-- +goose Down
DROP TABLE app.attempt_audio_plays;
