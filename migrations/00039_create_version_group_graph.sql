-- +goose Up
ALTER TABLE app.test_version_questions
  ADD CONSTRAINT tvq_section_identity_key UNIQUE USING INDEX tvq_section_identity_key;

CREATE TABLE app.test_version_groups (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_section_id uuid NOT NULL REFERENCES app.test_version_sections(id) ON DELETE CASCADE,
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200 AND btrim(title) <> ''),
  instructions jsonb CHECK (instructions IS NULL OR
    (jsonb_typeof(instructions)='object' AND (instructions->>'format'='semantic_v1') IS TRUE)),
  CONSTRAINT tvg_section_identity_key UNIQUE (id,test_version_section_id)
);
CREATE INDEX tvg_section_idx ON app.test_version_groups(test_version_section_id);

CREATE TABLE app.test_version_group_members (
  group_id uuid NOT NULL,
  test_version_section_id uuid NOT NULL,
  question_id uuid NOT NULL,
  ordinal smallint NOT NULL CHECK (ordinal BETWEEN 0 AND 199),
  option_order text NOT NULL CHECK (option_order IN ('shuffle','fixed')),
  PRIMARY KEY (group_id,question_id),
  CONSTRAINT tvgm_question_key UNIQUE (question_id),
  CONSTRAINT tvgm_ordinal_key UNIQUE (group_id,ordinal),
  CONSTRAINT tvgm_group_section_fk FOREIGN KEY (group_id,test_version_section_id)
    REFERENCES app.test_version_groups(id,test_version_section_id) ON DELETE CASCADE,
  CONSTRAINT tvgm_question_section_fk FOREIGN KEY (question_id,test_version_section_id)
    REFERENCES app.test_version_questions(id,test_version_section_id) ON DELETE CASCADE
);

CREATE TABLE app.test_version_units (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_section_id uuid NOT NULL REFERENCES app.test_version_sections(id) ON DELETE CASCADE,
  ordinal smallint NOT NULL CHECK (ordinal >= 0),
  question_id uuid,
  group_id uuid,
  CONSTRAINT tvu_kind CHECK ((question_id IS NULL) <> (group_id IS NULL)),
  CONSTRAINT tvu_question_section_fk FOREIGN KEY (question_id,test_version_section_id)
    REFERENCES app.test_version_questions(id,test_version_section_id) ON DELETE CASCADE,
  CONSTRAINT tvu_group_section_fk FOREIGN KEY (group_id,test_version_section_id)
    REFERENCES app.test_version_groups(id,test_version_section_id) ON DELETE CASCADE,
  CONSTRAINT tvu_ordinal_key UNIQUE (test_version_section_id,ordinal),
  CONSTRAINT tvu_question_key UNIQUE (question_id),
  CONSTRAINT tvu_group_key UNIQUE (group_id)
);

CREATE TABLE app.test_version_group_stimuli (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  group_id uuid NOT NULL REFERENCES app.test_version_groups(id) ON DELETE CASCADE,
  ordinal smallint NOT NULL CHECK (ordinal BETWEEN 0 AND 15),
  title text NOT NULL CHECK (length(title) BETWEEN 1 AND 200 AND btrim(title) <> ''),
  content jsonb NOT NULL CHECK ((jsonb_typeof(content)='object' AND content->>'format'='semantic_v1') IS TRUE),
  CONSTRAINT tvgs_group_identity_key UNIQUE (id,group_id),
  CONSTRAINT tvgs_ordinal_key UNIQUE (group_id,ordinal)
);

CREATE TABLE app.test_version_group_gap_bindings (
  stimulus_id uuid NOT NULL,
  group_id uuid NOT NULL,
  gap_id text NOT NULL CHECK (gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  kind text NOT NULL CHECK (kind IN ('question','blank')),
  question_id uuid NOT NULL,
  blank_gap_id text CHECK (blank_gap_id ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$'),
  PRIMARY KEY (stimulus_id,gap_id),
  CONSTRAINT tvgg_shape CHECK ((kind='blank')=(blank_gap_id IS NOT NULL)),
  CONSTRAINT tvgg_material_fk FOREIGN KEY (stimulus_id,group_id)
    REFERENCES app.test_version_group_stimuli(id,group_id) ON DELETE CASCADE,
  CONSTRAINT tvgg_member_fk FOREIGN KEY (group_id,question_id)
    REFERENCES app.test_version_group_members(group_id,question_id) ON DELETE CASCADE,
  CONSTRAINT tvgg_blank_fk FOREIGN KEY (question_id,blank_gap_id)
    REFERENCES app.test_version_blanks(test_version_question_id,gap_id) DEFERRABLE INITIALLY DEFERRED
);
CREATE UNIQUE INDEX tvgg_question_target_key ON app.test_version_group_gap_bindings(group_id,question_id) WHERE kind='question';
CREATE UNIQUE INDEX tvgg_blank_target_key ON app.test_version_group_gap_bindings(group_id,question_id,blank_gap_id) WHERE kind='blank';
CREATE INDEX tvgg_question_idx ON app.test_version_group_gap_bindings(question_id,blank_gap_id);

CREATE TABLE app.test_version_group_recordings (
  id uuid PRIMARY KEY DEFAULT uuidv7(),
  group_id uuid NOT NULL REFERENCES app.test_version_groups(id) ON DELETE CASCADE,
  media_asset_id uuid NOT NULL,
  media_asset_kind app.media_kind NOT NULL DEFAULT 'audio' CHECK (media_asset_kind='audio'),
  max_plays integer CHECK (max_plays IS NULL OR max_plays > 0),
  allow_seek boolean NOT NULL DEFAULT false,
  show_transcript_after_submit boolean NOT NULL DEFAULT false,
  transcript text CHECK (length(transcript)<=100000),
  CONSTRAINT tvgr_asset_fk FOREIGN KEY (media_asset_id,media_asset_kind)
    REFERENCES app.media_assets(id,kind) ON DELETE RESTRICT,
  CONSTRAINT tvgr_asset_key UNIQUE (group_id,media_asset_id),
  CONSTRAINT tvgr_binding_key UNIQUE (group_id,id,media_asset_id)
);
CREATE INDEX tvgr_asset_idx ON app.test_version_group_recordings(media_asset_id);

CREATE TABLE app.test_version_group_assets (
  stimulus_id uuid NOT NULL,
  group_id uuid NOT NULL,
  media_asset_id uuid NOT NULL,
  media_asset_kind app.media_kind NOT NULL,
  recording_id uuid,
  PRIMARY KEY (stimulus_id,media_asset_id),
  CONSTRAINT tvga_audio_binding CHECK ((media_asset_kind='audio')=(recording_id IS NOT NULL)),
  CONSTRAINT tvga_material_fk FOREIGN KEY (stimulus_id,group_id)
    REFERENCES app.test_version_group_stimuli(id,group_id) ON DELETE CASCADE,
  CONSTRAINT tvga_asset_fk FOREIGN KEY (media_asset_id,media_asset_kind)
    REFERENCES app.media_assets(id,kind) ON DELETE RESTRICT,
  CONSTRAINT tvga_recording_fk FOREIGN KEY (group_id,recording_id,media_asset_id)
    REFERENCES app.test_version_group_recordings(group_id,id,media_asset_id) ON DELETE CASCADE
);
CREATE INDEX tvga_asset_idx ON app.test_version_group_assets(media_asset_id);
CREATE INDEX tvga_recording_idx ON app.test_version_group_assets(group_id,recording_id,media_asset_id);

REVOKE UPDATE ON app.test_version_groups, app.test_version_group_members,
  app.test_version_units, app.test_version_group_stimuli,
  app.test_version_group_gap_bindings, app.test_version_group_recordings,
  app.test_version_group_assets FROM quizzivy_app;

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.test_version_groups) OR EXISTS (SELECT 1 FROM app.test_version_units) THEN
    RAISE EXCEPTION 'frozen group storage cannot be removed after snapshot writes';
  END IF;
END;
$$;
-- +goose StatementEnd
DROP TABLE app.test_version_group_assets;
DROP TABLE app.test_version_group_recordings;
DROP TABLE app.test_version_group_gap_bindings;
DROP TABLE app.test_version_group_stimuli;
DROP TABLE app.test_version_units;
DROP TABLE app.test_version_group_members;
DROP TABLE app.test_version_groups;
ALTER TABLE app.test_version_questions DROP CONSTRAINT tvq_section_identity_key;
CREATE UNIQUE INDEX tvq_section_identity_key ON app.test_version_questions(id,test_version_section_id);
