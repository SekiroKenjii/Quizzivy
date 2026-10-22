-- +goose Up

-- Matching (T-R6.1b). The choices are the question's options, kept in their
-- order because authors refer to their keys, and none is marked correct; the
-- items are rows here, each keyed to one choice. Marking is per item.
--
-- The key is a composite FK onto 00151's (question_id, id) index, so it cannot
-- point into another question. NO ACTION rather than RESTRICT: NO ACTION is
-- checked at the end of the statement, so a purge of the question, which
-- cascades to its options and its items together, passes whatever order the
-- cascade visits them in, while deleting an option an item still names fails.
--
-- The ALTERs below take ACCESS EXCLUSIVE on both question tables, so
-- lock_timeout fails the file at once rather than queue writers behind it.
SET LOCAL lock_timeout = '5s';

-- [D-13] The ordinal key is deferrable for the editor's reorder.
CREATE TABLE app.question_match_items (
  id                uuid PRIMARY KEY DEFAULT uuidv7(),
  question_id       uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal           smallint NOT NULL CHECK (ordinal >= 0),
  text              text NOT NULL CHECK (length(text) BETWEEN 1 AND 500 AND btrim(text) <> ''),
  correct_option_id uuid NOT NULL,

  CONSTRAINT question_match_items_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE,
  CONSTRAINT question_match_items_option_fk FOREIGN KEY (question_id, correct_option_id)
    REFERENCES app.question_options (question_id, id) ON DELETE NO ACTION
);

-- Serves the FK's check when an option is deleted.
CREATE INDEX question_match_items_option_idx
  ON app.question_match_items (question_id, correct_option_id);

-- The frozen copy, against the frozen options. Never updated, as 00150's.
CREATE TABLE app.test_version_match_items (
  id                       uuid PRIMARY KEY DEFAULT uuidv7(),
  test_version_question_id uuid NOT NULL
    REFERENCES app.test_version_questions(id) ON DELETE CASCADE,
  ordinal                  smallint NOT NULL CHECK (ordinal >= 0),
  text                     text NOT NULL CHECK (length(text) BETWEEN 1 AND 500 AND btrim(text) <> ''),
  correct_option_id        uuid NOT NULL,

  CONSTRAINT test_version_match_items_ordinal_key UNIQUE (test_version_question_id, ordinal),
  CONSTRAINT test_version_match_items_option_fk FOREIGN KEY (test_version_question_id, correct_option_id)
    REFERENCES app.test_version_options (test_version_question_id, id) ON DELETE NO ACTION
);

CREATE INDEX test_version_match_items_option_idx
  ON app.test_version_match_items (test_version_question_id, correct_option_id);

REVOKE UPDATE ON app.test_version_match_items FROM quizzivy_app;

-- Whether two items may share a choice. Present exactly on a matching question,
-- so it says something on every row that has it. Both tables hold rows, so the
-- CHECK is added NOT VALID and validated, as 00146 does. A matching row can
-- exist from 00145 on, so each table is backfilled with the strict reading
-- (false) before its VALIDATE, and the file applies wherever 00145 has run.
ALTER TABLE app.questions ADD COLUMN match_allow_reuse boolean;
UPDATE app.questions SET match_allow_reuse = false WHERE type = 'matching';
ALTER TABLE app.questions
  ADD CONSTRAINT questions_match_allow_reuse_iff_matching
    CHECK ((type = 'matching') = (match_allow_reuse IS NOT NULL))
    NOT VALID;
ALTER TABLE app.questions VALIDATE CONSTRAINT questions_match_allow_reuse_iff_matching;

ALTER TABLE app.test_version_questions ADD COLUMN match_allow_reuse boolean;
UPDATE app.test_version_questions SET match_allow_reuse = false WHERE type = 'matching';
ALTER TABLE app.test_version_questions
  ADD CONSTRAINT test_version_questions_match_allow_reuse_iff_matching
    CHECK ((type = 'matching') = (match_allow_reuse IS NOT NULL))
    NOT VALID;
ALTER TABLE app.test_version_questions VALIDATE CONSTRAINT test_version_questions_match_allow_reuse_iff_matching;

-- +goose Down

SET LOCAL lock_timeout = '5s';

-- Refuses while a matching question exists in either table (QC-R6B-1): without
-- its items and its reuse rule a published matching question can never be
-- marked again, so the Down would destroy a frozen answer key. 00145's Down
-- refuses for the same rows, but only after this file has run.
-- +goose StatementBegin
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM app.questions WHERE type = 'matching')
     OR EXISTS (SELECT 1 FROM app.test_version_questions WHERE type = 'matching')
     OR EXISTS (SELECT 1 FROM app.question_match_items)
     OR EXISTS (SELECT 1 FROM app.test_version_match_items) THEN
    RAISE EXCEPTION 'matching questions exist; dropping their items would lose their answer keys';
  END IF;
END
$$;
-- +goose StatementEnd

-- The indexes the FKs use stay for 00151's Down.
ALTER TABLE app.test_version_questions DROP COLUMN match_allow_reuse;
ALTER TABLE app.questions DROP COLUMN match_allow_reuse;
DROP TABLE app.test_version_match_items;
DROP TABLE app.question_match_items;
