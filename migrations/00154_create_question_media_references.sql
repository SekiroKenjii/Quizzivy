-- +goose Up

-- Media inside a question's prose (DG-109, R6-12, D-064 ruling 4): every image
-- or audio block of a prompt or an explanation that names an asset is a row
-- here, written in the same transaction as the content. "Used in", the delete
-- check, the unreferenced sweep and a student's reach read these rows, as they
-- read group_stimulus_assets and test_version_group_assets, rather than scan
-- the JSON. The prompt's audio marker names no asset (it places the question's
-- own audio, media_asset_id), so it has no row. The composite key onto
-- media_assets (id, kind) refuses an image block naming a recording, and
-- RESTRICT stops an asset row going while a row names it.
-- The foreign keys lock the tables they reference while the file runs, so
-- lock_timeout fails it at once rather than queue writers behind it.
SET LOCAL lock_timeout = '5s';

CREATE TABLE app.question_media_references (
  question_id      uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  field            text NOT NULL CHECK (field IN ('prompt', 'explanation')),
  media_asset_id   uuid NOT NULL,
  media_asset_kind app.media_kind NOT NULL,
  PRIMARY KEY (question_id, field, media_asset_id),
  CONSTRAINT question_media_references_media FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets(id, kind) ON DELETE RESTRICT
);
CREATE INDEX question_media_references_media_idx ON app.question_media_references (media_asset_id);

-- The frozen copy, written at publish (R6-13) and never changed: the app role
-- may not UPDATE it, as 00039 does for the frozen group graph.
CREATE TABLE app.test_version_question_media_references (
  test_version_question_id uuid NOT NULL REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  field                    text NOT NULL CHECK (field IN ('prompt', 'explanation')),
  media_asset_id           uuid NOT NULL,
  media_asset_kind         app.media_kind NOT NULL,
  PRIMARY KEY (test_version_question_id, field, media_asset_id),
  CONSTRAINT tvq_media_references_media FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets(id, kind) ON DELETE RESTRICT
);
CREATE INDEX tvq_media_references_media_idx ON app.test_version_question_media_references (media_asset_id);

REVOKE UPDATE ON app.test_version_question_media_references FROM quizzivy_app;

-- +goose Down

SET LOCAL lock_timeout = '5s';

-- Refuses while a reference exists (QC-R6M-1): without its rows a published
-- paper's image could be deleted while the paper still draws it.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.question_media_references)
     OR EXISTS (SELECT 1 FROM app.test_version_question_media_references) THEN
    RAISE EXCEPTION 'question media references exist; dropping them would let a referenced asset be deleted';
  END IF;
END
$$;
-- +goose StatementEnd

DROP TABLE app.test_version_question_media_references;
DROP TABLE app.question_media_references;
