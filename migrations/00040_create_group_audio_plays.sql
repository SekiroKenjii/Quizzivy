-- +goose Up
CREATE TABLE app.attempt_group_audio_plays (
  attempt_id uuid NOT NULL REFERENCES app.attempts(id) ON DELETE CASCADE,
  recording_id uuid NOT NULL REFERENCES app.test_version_group_recordings(id) ON DELETE RESTRICT,
  plays integer NOT NULL CHECK (plays > 0),
  last_played_at timestamptz NOT NULL,
  PRIMARY KEY (attempt_id,recording_id)
);
CREATE INDEX agap_recording_idx ON app.attempt_group_audio_plays(recording_id);

CREATE TABLE app.attempt_group_audio_receipts (
  attempt_id uuid NOT NULL,
  play_id uuid NOT NULL,
  recording_id uuid NOT NULL,
  session_id uuid NOT NULL,
  received_at timestamptz NOT NULL,
  PRIMARY KEY (attempt_id,play_id),
  CONSTRAINT agar_counter_fk FOREIGN KEY (attempt_id,recording_id)
    REFERENCES app.attempt_group_audio_plays(attempt_id,recording_id) ON DELETE CASCADE
);
CREATE INDEX agar_counter_idx ON app.attempt_group_audio_receipts(attempt_id,recording_id);
REVOKE UPDATE, DELETE ON app.attempt_group_audio_receipts FROM quizzivy_app;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.attempt_group_audio_plays) THEN
    RAISE EXCEPTION 'shared playback storage cannot be removed after listening events';
  END IF;
END;
$$;
-- +goose StatementEnd
DROP TABLE app.attempt_group_audio_receipts;
DROP TABLE app.attempt_group_audio_plays;
