-- +goose Up

CREATE TABLE app.tests (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  title           text NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
  description     text,
  status          app.test_status NOT NULL DEFAULT 'draft',

  -- 0 means "never published". The CHECK below is what keeps that honest.
  current_version integer NOT NULL DEFAULT 0 CHECK (current_version >= 0),

  created_by      uuid NOT NULL REFERENCES app.users(id) ON DELETE RESTRICT,
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,

  -- Makes §7's `currentVersion` honest: a test cannot be published or archived
  -- with no snapshot behind it, which is the state that would let a student
  -- open an assignment pointing at nothing.
  CONSTRAINT tests_published_has_version
    CHECK (status = 'draft' OR current_version > 0)
);

CREATE INDEX tests_status_id_idx
  ON app.tests (status, id DESC) WHERE deleted_at IS NULL;

CREATE TRIGGER tests_set_updated_at BEFORE UPDATE ON app.tests
  FOR EACH ROW EXECUTE FUNCTION app.set_updated_at();

-- [D-14] The DRAFT structure. §13.3 goes straight from `tests` to the version
-- tables, leaving nowhere for the builder to autosave into before a publish.
CREATE TABLE app.test_sections (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),

  -- CASCADE: a section is owned by its test and is meaningless without it.
  test_id      uuid NOT NULL REFERENCES app.tests(id) ON DELETE CASCADE,
  ordinal      smallint NOT NULL CHECK (ordinal >= 0),
  title        text NOT NULL CHECK (title <> ''),
  instructions text,

  -- [D-13] Deferrable because the builder reorders sections by drag-and-drop,
  -- writing the new ordinals one row at a time.
  CONSTRAINT test_sections_ordinal_key UNIQUE (test_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE
);

CREATE TABLE app.test_section_questions (
  id              uuid PRIMARY KEY DEFAULT uuidv7(),
  test_section_id uuid NOT NULL REFERENCES app.test_sections(id) ON DELETE CASCADE,
  ordinal         smallint NOT NULL CHECK (ordinal >= 0),

  -- RESTRICT, not CASCADE: a draft REFERENCES a bank question, it does not own
  -- one. The bank soft-deletes, so this fires only on a hard purge -- which is
  -- exactly when someone should be told a draft still uses it.
  question_id     uuid NOT NULL REFERENCES app.questions(id) ON DELETE RESTRICT,

  CONSTRAINT test_section_questions_ordinal_key UNIQUE (test_section_id, ordinal)
    DEFERRABLE INITIALLY IMMEDIATE,

  -- One question cannot appear twice in a section: the two copies would be
  -- indistinguishable to a student and would score twice.
  CONSTRAINT test_section_questions_no_dupes UNIQUE (test_section_id, question_id)
);

-- Postgres does not index a foreign key's own column, so without this the
-- RESTRICT above makes every question delete scan and lock this table. It also
-- serves §8's "where used" on the bank.
CREATE INDEX test_section_questions_question_idx
  ON app.test_section_questions (question_id);

-- +goose Down

DROP TABLE IF EXISTS app.test_section_questions;
DROP TABLE IF EXISTS app.test_sections;
DROP TABLE IF EXISTS app.tests;
