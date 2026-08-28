-- +goose Up

-- The question bank (§7, §8). One row per authored question; the four child
-- tables in 00012 and 00013 carry the per-type detail.
CREATE TABLE app.questions (
  id                          uuid PRIMARY KEY DEFAULT uuidv7(),
  type                        app.question_type NOT NULL,
  prompt                      text NOT NULL CHECK (prompt <> ''),

  -- [D-05] The composite FK's two halves. Carrying the kind alongside the id
  -- is what lets the audio-policy CHECK below be enforced relationally: without
  -- it, "audio policy iff the asset is audio" would need a query, and a CHECK
  -- cannot run one.
  media_asset_id              uuid,
  media_asset_kind            app.media_kind,

  -- [D-04] Nullable, against §13.3's NOT NULL DEFAULT. §7 types the policy as
  -- `audio?: AudioPolicy` -- present iff the asset is audio -- and a default of
  -- false gives every text question a meaningless allow_seek that the API layer
  -- then has to decide when to hide. Nullable plus the biconditional below puts
  -- that knowledge in the database. §11.1's authoring defaults (maxPlays 2,
  -- allowSeek false, showTranscriptAfterSubmit true) belong to the editor, not
  -- to storage.
  audio_max_plays             integer CHECK (audio_max_plays IS NULL
                                             OR audio_max_plays > 0),
  audio_allow_seek            boolean,
  audio_show_transcript_after boolean,
  transcript                  text,

  points                      numeric(8,2) NOT NULL
                                CHECK (points > 0 AND points <= 999999.99),
  explanation                 text,
  sample_answer               text,
  tags                        text[] NOT NULL DEFAULT '{}',

  -- RESTRICT: a question's provenance must survive the author's user row.
  created_by                  uuid NOT NULL
                                REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at                  timestamptz NOT NULL DEFAULT now(),
  updated_at                  timestamptz NOT NULL DEFAULT now(),

  -- Soft delete (§13.2): a published version may still reference this row.
  deleted_at                  timestamptz,

  -- RESTRICT rather than SET NULL: dropping the asset out from under a question
  -- would leave a listening question with nothing to listen to.
  FOREIGN KEY (media_asset_id, media_asset_kind)
    REFERENCES app.media_assets (id, kind) ON DELETE RESTRICT,

  -- Both halves of the composite FK or neither. A half-set pair would make the
  -- FK vacuous, since a NULL component means the constraint is not checked.
  CONSTRAINT questions_media_pair_complete
    CHECK ((media_asset_id IS NULL) = (media_asset_kind IS NULL)),

  -- [D-04] §7's "present iff kind === 'audio'", in both directions and in one
  -- line. IS NOT DISTINCT FROM rather than =, because media_asset_kind is NULL
  -- on a question with no asset at all and `NULL = 'audio'` is NULL, which a
  -- CHECK treats as passing -- so plain equality would silently permit an audio
  -- policy on a question with no media.
  CONSTRAINT questions_audio_policy_iff_audio
    CHECK ((media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind)
           = (audio_allow_seek IS NOT NULL
              AND audio_show_transcript_after IS NOT NULL)),

  -- A transcript is a transcript OF something. Not a biconditional: audio
  -- without a transcript is normal.
  CONSTRAINT questions_transcript_iff_audio
    CHECK (transcript IS NULL
           OR media_asset_kind IS NOT DISTINCT FROM 'audio'::app.media_kind),

  -- §7 marks sample_answer short_answer-only and ADMIN ONLY. The explicit
  -- column lists in the student projection keep it out of payloads; this keeps
  -- it from being SET on a type whose grading UI would never show it.
  CONSTRAINT questions_sample_answer_only_short_answer
    CHECK (sample_answer IS NULL OR type = 'short_answer')
);

CREATE TRIGGER questions_set_updated_at BEFORE UPDATE ON app.questions
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- [D-12] Every bank query filters deleted rows, so the partial predicate costs
-- nothing and keeps the indexes smaller.
CREATE INDEX questions_tags_idx
  ON app.questions USING gin (tags) WHERE deleted_at IS NULL;

-- Word search. The two-argument to_tsvector with a literal config is IMMUTABLE
-- and therefore legal in an expression index; the one-argument form is STABLE
-- and would be rejected.
CREATE INDEX questions_prompt_fts_idx
  ON app.questions USING gin (to_tsvector('simple', prompt)) WHERE deleted_at IS NULL;

-- [D-11] Substring and accent-insensitive search, which the tsvector cannot do:
-- 'simple' does no diacritic folding, so `nghe` would never match `nghé` in a
-- Vietnamese-first product. pg_trgm alone does not fix it either -- it is
-- case-insensitive but NOT accent-insensitive -- hence the explicit fold.
--
-- Queries MUST spell the expression identically or the planner will not use
-- this index:
--   WHERE app.immutable_unaccent(lower(prompt))
--         LIKE '%' || app.immutable_unaccent(lower($1)) || '%'
CREATE INDEX questions_prompt_trgm_idx
  ON app.questions USING gin (app.immutable_unaccent(lower(prompt)) gin_trgm_ops)
  WHERE deleted_at IS NULL;

-- "Which BANK questions use this asset". NOT the delete-blocking check in §8:
-- that one asks whether a PUBLISHED VERSION references the asset, which is a
-- query against test_version_questions served by tvq_media_idx (00017). Both
-- are wanted; they answer different questions.
CREATE INDEX questions_media_idx
  ON app.questions (media_asset_id) WHERE media_asset_id IS NOT NULL;

-- §8's type filter and §13.8's keyset pagination in one index. uuidv7 PKs make
-- `id DESC` a valid recency order.
CREATE INDEX questions_type_id_idx
  ON app.questions (type, id DESC) WHERE deleted_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS app.questions;
