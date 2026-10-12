-- +goose Up

-- The pause rule frozen with a published question (T-D4.13, DG-183): publish
-- copies it from app.questions, so a later edit of the bank question never
-- reaches a version. Nullable, no default, no backfill, for the reasons of
-- 00107: a version published before this column, or by the previous binary,
-- reads NULL, which the readers take as true.
ALTER TABLE app.test_version_questions
  ADD COLUMN audio_allow_pause boolean
    CONSTRAINT tvq_allow_pause_only_with_audio
    CHECK (audio_allow_pause IS NULL
           OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind);

-- +goose Down

ALTER TABLE app.test_version_questions
  DROP CONSTRAINT tvq_allow_pause_only_with_audio,
  DROP COLUMN audio_allow_pause;
