-- +goose Up

-- Whether a student may pause this question's audio once it is playing
-- (T-D4.13, DG-183). Nullable with no default and no backfill, so the previous
-- release's insert, which names no such column, keeps working during the
-- rolling deploy; NULL is read as true (COALESCE(audio_allow_pause, true)), the
-- value every audio question had before the control existed. It is present
-- only with audio, in the one direction that 00014 uses for audio_max_plays.
-- questions_audio_policy_iff_audio is not widened: the previous binary writes
-- no value here, and a biconditional over this column would refuse its rows.
ALTER TABLE app.questions
  ADD COLUMN audio_allow_pause boolean
    CONSTRAINT questions_allow_pause_only_with_audio
    CHECK (audio_allow_pause IS NULL
           OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind);

-- +goose Down

ALTER TABLE app.questions
  DROP CONSTRAINT questions_allow_pause_only_with_audio,
  DROP COLUMN audio_allow_pause;
