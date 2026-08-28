-- +goose Up

-- [D-04] The half of the audio-policy rule that 00011 left open.
--
-- `questions_audio_policy_iff_audio` is a biconditional over
-- `audio_allow_seek` and `audio_show_transcript_after`, and does not mention
-- `audio_max_plays` at all.
--
-- Leaving it out of the RIGHT-hand side is correct and deliberate:
-- AudioPolicy.maxPlays is `[integer, 'null']` in the contract, where null means
-- unlimited, so a NULL column on an audio question is a meaningful value rather
-- than an absent one. Requiring it to be non-NULL would forbid "unlimited".
--
-- The other direction was simply missing. On a question with no media,
-- `media_asset_kind` is NULL so the left side is false, which the biconditional
-- satisfies as soon as ONE of the two booleans is NULL -- and `audio_max_plays`
-- was then unconstrained. `audio_max_plays = 3` on a `short_answer` question
-- satisfied every CHECK on the table.
--
-- Nothing emits a partial policy today, because the API cannot assemble an
-- AudioPolicy without the two booleans. It is added anyway for the reason this
-- table encodes rules as CHECKs at all, in 00010's words: these "keep the rule
-- true if a code path is added later that skips this one". T-2.6's question
-- editor is exactly that code path, and it is next.
--
-- One-directional, the same shape as `questions_transcript_iff_audio` beside
-- it, and for the same reason: absence is legal, presence-without-audio is not.
ALTER TABLE app.questions
  ADD CONSTRAINT questions_max_plays_only_with_audio
  CHECK (audio_max_plays IS NULL
         OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind);

-- +goose Down

ALTER TABLE app.questions
  DROP CONSTRAINT IF EXISTS questions_max_plays_only_with_audio;
