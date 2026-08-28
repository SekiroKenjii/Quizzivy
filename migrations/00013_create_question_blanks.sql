-- +goose Up

-- fill_blank questions (§7). §13.3 says only that these "mirror" the options
-- table; the shape below is this document's.
CREATE TABLE app.question_blanks (
  id             uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id    uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,

  -- 1-indexed, not 0. Blanks are addressed from the prompt Markdown by a
  -- placeholder ({{1}}, {{2}} -- the resolution in 40-open-items.md), so a
  -- 0-ordinal blank would be unreachable from the text that references it.
  -- §7's blanks[].ordinal is silent on the base; this pins it.
  ordinal        smallint NOT NULL CHECK (ordinal >= 1),
  case_sensitive boolean NOT NULL DEFAULT false,

  -- [D-13] Deferrable for the same reason as question_options: the editor
  -- reorders blanks.
  CONSTRAINT question_blanks_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

-- The accepted answers for one blank. Several per blank is the normal case:
-- "colour"/"color", "is not"/"isn't".
CREATE TABLE app.question_blank_answers (
  id       uuid PRIMARY KEY DEFAULT uuidv7(),
  blank_id uuid NOT NULL REFERENCES app.question_blanks(id) ON DELETE CASCADE,
  answer   text NOT NULL CHECK (answer <> ''),

  -- Exact, deliberately NOT lower(answer): a case_sensitive blank legitimately
  -- wants `Cat` and `cat` as two distinct accepted answers. Case-insensitive
  -- matching happens at grading time, against the blank's case_sensitive flag.
  UNIQUE (blank_id, answer)
);

-- Publish validation (§8) enforces "at least one accepted answer per blank" and
-- that the prompt's placeholder set equals the blank ordinal set. Neither is a
-- CHECK -- both span tables -- so both live in the publish routine (T-2.10).

-- +goose Down

DROP TABLE IF EXISTS app.question_blank_answers;
DROP TABLE IF EXISTS app.question_blanks;
