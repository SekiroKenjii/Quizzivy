-- +goose Up

-- Choice options for single_choice, multi_choice, and true_false (§7).
CREATE TABLE app.question_options (
  id          uuid PRIMARY KEY DEFAULT uuidv7(),

  -- CASCADE: an option is a true owned child, meaningless without its question
  -- (§13.2). The bank soft-deletes, so this fires only on a hard purge.
  question_id uuid NOT NULL REFERENCES app.questions(id) ON DELETE CASCADE,
  ordinal     smallint NOT NULL CHECK (ordinal >= 0),
  text        text NOT NULL CHECK (text <> ''),
  is_correct  boolean NOT NULL DEFAULT false,

  -- [D-13] DEFERRABLE because the builder reorders by drag-and-drop (§8). A
  -- reorder transaction issues
  --   SET CONSTRAINTS app.question_options_ordinal_key DEFERRED
  -- and writes the new ordinals directly, instead of the two-phase
  -- negative-offset trick that exists only to dodge the check.
  --
  -- Which reorder needs it, measured on 18.6 rather than assumed:
  -- 20-data-model.md says a single `UPDATE ... SET ordinal = (ordinal+1) % 3`
  -- transiently violates uniqueness. It does not -- Postgres checks a unique
  -- constraint at END OF STATEMENT, so a set-based permutation is fine with no
  -- deferral. What needs deferral is the MULTI-statement reorder, which is the
  -- shape a real one takes: the client sends (id, ordinal) pairs and the server
  -- writes them a row at a time, so the first write collides with a row that has
  -- not moved yet. See db/reorder_test.go, which pins both halves.
  --
  -- INITIALLY IMMEDIATE so every OTHER statement still gets the error at the
  -- point it happens, where it can be attributed to the row that caused it.
  CONSTRAINT question_options_ordinal_key UNIQUE (question_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

-- No separate (question_id) index: the unique constraint's index has it as a
-- leading column and serves every lookup by question.

-- +goose Down

DROP TABLE IF EXISTS app.question_options;
