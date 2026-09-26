-- +goose Up
ALTER TABLE app.questions
  ADD CONSTRAINT questions_context_identity_key UNIQUE USING INDEX questions_context_identity_key,
  ADD CONSTRAINT questions_context_ordinal_key UNIQUE USING INDEX questions_context_ordinal_key
    DEFERRABLE INITIALLY IMMEDIATE;

CREATE TABLE app.test_section_units (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  test_section_id uuid NOT NULL REFERENCES app.test_sections(id) ON DELETE CASCADE,
  ordinal smallint NOT NULL CHECK (ordinal >= 0),
  question_id uuid REFERENCES app.questions(id) ON DELETE RESTRICT,
  group_id uuid,
  CONSTRAINT test_section_units_kind CHECK ((question_id IS NULL) <> (group_id IS NULL)),
  CONSTRAINT test_section_units_group_owner FOREIGN KEY (group_id, test_section_id)
    REFERENCES app.question_groups(id, owner_section_id) ON DELETE RESTRICT,
  CONSTRAINT test_section_units_ordinal_key UNIQUE (test_section_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE,
  CONSTRAINT test_section_units_question_key UNIQUE (test_section_id, question_id),
  CONSTRAINT test_section_units_group_key UNIQUE (group_id)
);
CREATE INDEX test_section_units_question_idx ON app.test_section_units (question_id)
  WHERE question_id IS NOT NULL;

CREATE TABLE app.group_stimuli (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  group_id uuid NOT NULL REFERENCES app.question_groups(id) ON DELETE CASCADE,
  ordinal smallint NOT NULL CHECK (ordinal BETWEEN 0 AND 15),
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200 AND btrim(title) <> ''),
  content jsonb NOT NULL CHECK (jsonb_typeof(content) = 'object'
    AND (content->>'format' IN ('semantic_v1', 'legacy_markdown_v1')) IS TRUE),
  CONSTRAINT group_stimuli_owner_key UNIQUE (id, group_id),
  CONSTRAINT group_stimuli_ordinal_key UNIQUE (group_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE app.group_gap_bindings (
  stimulus_id uuid NOT NULL,
  group_id uuid NOT NULL,
  gap_id text NOT NULL CHECK (gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  kind text NOT NULL CHECK (kind IN ('question', 'blank')),
  question_id uuid NOT NULL,
  blank_gap_id text CHECK (blank_gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  PRIMARY KEY (stimulus_id, gap_id),
  CONSTRAINT group_gap_bindings_kind CHECK ((kind = 'blank') = (blank_gap_id IS NOT NULL)),
  CONSTRAINT group_gap_bindings_material FOREIGN KEY (stimulus_id, group_id)
    REFERENCES app.group_stimuli(id, group_id) ON DELETE CASCADE,
  CONSTRAINT group_gap_bindings_member FOREIGN KEY (group_id, question_id)
    REFERENCES app.questions(context_group_id, id) ON DELETE NO ACTION
    DEFERRABLE INITIALLY DEFERRED,
  CONSTRAINT group_gap_bindings_blank FOREIGN KEY (question_id, blank_gap_id)
    REFERENCES app.question_blanks(question_id, gap_id) ON DELETE NO ACTION
    DEFERRABLE INITIALLY DEFERRED
);
CREATE UNIQUE INDEX group_gap_bindings_choice_key ON app.group_gap_bindings (group_id, question_id)
  WHERE kind = 'question';
CREATE UNIQUE INDEX group_gap_bindings_blank_key ON app.group_gap_bindings (group_id, question_id, blank_gap_id)
  WHERE kind = 'blank';
CREATE INDEX group_gap_bindings_question_idx ON app.group_gap_bindings (question_id, blank_gap_id);

CREATE TABLE app.group_recordings (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  group_id uuid NOT NULL REFERENCES app.question_groups(id) ON DELETE CASCADE,
  media_asset_id uuid NOT NULL,
  media_asset_kind app.media_kind NOT NULL DEFAULT 'audio' CHECK (media_asset_kind = 'audio'),
  max_plays integer CHECK (max_plays > 0),
  allow_seek boolean NOT NULL,
  show_transcript_after_submit boolean NOT NULL,
  transcript text CHECK (length(transcript) <= 100000),
  CONSTRAINT group_recordings_media FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets(id, kind) ON DELETE RESTRICT,
  CONSTRAINT group_recordings_asset_key UNIQUE (group_id, media_asset_id),
  CONSTRAINT group_recordings_binding_key UNIQUE (group_id, id, media_asset_id)
);
CREATE INDEX group_recordings_media_idx ON app.group_recordings (media_asset_id);

CREATE TABLE app.group_stimulus_assets (
  stimulus_id uuid NOT NULL,
  group_id uuid NOT NULL,
  media_asset_id uuid NOT NULL,
  media_asset_kind app.media_kind NOT NULL,
  recording_id uuid,
  PRIMARY KEY (stimulus_id, media_asset_id),
  CONSTRAINT group_stimulus_assets_kind CHECK ((media_asset_kind = 'audio') = (recording_id IS NOT NULL)),
  CONSTRAINT group_stimulus_assets_material FOREIGN KEY (stimulus_id, group_id)
    REFERENCES app.group_stimuli(id, group_id) ON DELETE CASCADE,
  CONSTRAINT group_stimulus_assets_media FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets(id, kind) ON DELETE RESTRICT,
  CONSTRAINT group_stimulus_assets_recording FOREIGN KEY (group_id, recording_id, media_asset_id)
    REFERENCES app.group_recordings(group_id, id, media_asset_id) ON DELETE NO ACTION
    DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX group_stimulus_assets_media_idx ON app.group_stimulus_assets (media_asset_id);
CREATE INDEX group_stimulus_assets_recording_idx ON app.group_stimulus_assets (group_id, recording_id, media_asset_id)
  WHERE recording_id IS NOT NULL;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.question_groups) OR EXISTS (SELECT 1 FROM app.test_section_units) THEN
    RAISE EXCEPTION 'question group graph cannot be removed after group writes';
  END IF;
END;
$$;
-- +goose StatementEnd

DROP TABLE app.group_stimulus_assets;
DROP TABLE app.group_recordings;
DROP TABLE app.group_gap_bindings;
DROP TABLE app.group_stimuli;
DROP TABLE app.test_section_units;
ALTER TABLE app.questions
  DROP CONSTRAINT questions_context_ordinal_key,
  DROP CONSTRAINT questions_context_identity_key;
CREATE UNIQUE INDEX questions_context_identity_key ON app.questions (context_group_id, id);
CREATE UNIQUE INDEX questions_context_ordinal_key ON app.questions (context_group_id, context_ordinal);
