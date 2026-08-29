-- +goose Up

-- The frozen copy of each question, carrying its own grading key. Nothing here
-- reads through to the bank at attempt time.
CREATE TABLE app.test_version_questions (
  id                          uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_section_id     uuid NOT NULL
    REFERENCES app.test_version_sections(id) ON DELETE CASCADE,
  ordinal                     smallint NOT NULL CHECK (ordinal >= 0),

  -- [D-07] SET NULL, deviating from §13.2's RESTRICT default. The column is
  -- informational -- it powers "which bank question did this come from" -- and
  -- RESTRICT would let a three-year-old frozen version veto a bank cleanup
  -- forever. Losing it degrades a link, not an attempt.
  source_question_id          uuid
    REFERENCES app.questions(id) ON DELETE SET NULL,

  type                        app.question_type NOT NULL,
  prompt                      text NOT NULL,
  media_asset_id              uuid,
  media_asset_kind            app.media_kind,
  audio_max_plays             integer CHECK (audio_max_plays IS NULL
                                             OR audio_max_plays > 0),
  audio_allow_seek            boolean,
  audio_show_transcript_after boolean,
  transcript                  text,
  points                      numeric(8,2) NOT NULL CHECK (points > 0),
  explanation                 text,
  sample_answer               text,

  UNIQUE (test_version_section_id, ordinal),

  -- RESTRICT: an asset a published version depends on must not vanish under it.
  FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets (id, kind) ON DELETE RESTRICT,

  CONSTRAINT tvq_media_pair_complete
    CHECK ((media_asset_id IS NULL) = (media_asset_kind IS NULL)),
  CONSTRAINT tvq_audio_policy_iff_audio
    CHECK ((media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind)
           = (audio_allow_seek IS NOT NULL
              AND audio_show_transcript_after IS NOT NULL)),
  CONSTRAINT tvq_max_plays_only_with_audio
    CHECK (audio_max_plays IS NULL
           OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind)
);

-- Load-bearing, not decorative: this is the index behind
-- DELETE /admin/media/:id returning 409 when a published version uses the
-- asset. Without it that check scans every version question on every attempt.
CREATE INDEX tvq_media_idx
  ON app.test_version_questions (media_asset_id) WHERE media_asset_id IS NOT NULL;

-- Serves the reverse lookup and keeps the SET NULL above from scanning.
CREATE INDEX tvq_source_idx
  ON app.test_version_questions (source_question_id)
  WHERE source_question_id IS NOT NULL;

CREATE TABLE app.test_version_options (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 0),
  text                     text NOT NULL,
  is_correct               boolean NOT NULL DEFAULT false,

  UNIQUE (test_version_question_id, ordinal)
);

CREATE TABLE app.test_version_blanks (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 1),
  case_sensitive           boolean NOT NULL DEFAULT false,

  UNIQUE (test_version_question_id, ordinal)
);

CREATE TABLE app.test_version_blank_answers (
  id                    uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_blank_id uuid NOT NULL
    REFERENCES app.test_version_blanks(id) ON DELETE CASCADE,
  answer                text NOT NULL,

  UNIQUE (test_version_blank_id, answer)
);

-- +goose Down

DROP TABLE IF EXISTS app.test_version_blank_answers;
DROP TABLE IF EXISTS app.test_version_blanks;
DROP TABLE IF EXISTS app.test_version_options;
DROP TABLE IF EXISTS app.test_version_questions;
