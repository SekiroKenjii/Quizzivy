-- +goose Up

-- F-37: the text a person typed is stored composed (Unicode Normalization Form C)
-- from this release on; this brings the rows written before it to the same form.
-- Be Vietnam Pro draws precomposed Vietnamese letters, so decomposed text (Unikey's
-- "Unicode tổ hợp", a file name from a Mac, a paste from a PDF) fell back to another
-- font in a title, a name or a note.
--
-- What is left alone, and why:
--   test_version_*, test_versions.change_note  published versions are frozen.
--   attempt_answers                            a student's saved work is history; new
--                                              saves are composed by the server.
--   attempt_events, audit_log                  append-only.
--   every prose document (prompt_content, explanation_content, question_options.content,
--     question_groups.instructions, group_stimuli.content) and the plain text kept
--     beside one (questions.prompt, questions.explanation, question_options.text where
--     a document exists): a write checks that the text equals the document's projection,
--     SQL cannot derive that projection, and composing the two apart breaks the equality
--     where a combining mark opens a text node. The next save of such a row composes the
--     pair, and the web composes it where it is drawn. Rows that have no document are
--     rewritten below.
--   emails, passwords, tokens, join codes, ids, keys, kinds, statuses and URLs.
--
-- Lock and duration. One transaction, no batching, no rewrite of a table and no index
-- build. A plain UPDATE would also move updated_at, because app.set_updated_at() sets
-- it unconditionally, and tests.updated_at is the autosave token an open editor holds,
-- the lists sort on it and word_imports.updated_at is the idle-retention clock. So each
-- of the eight tables that has such a trigger has that one trigger, by name, disabled
-- for the statements below and enabled again before the transaction ends. DISABLE
-- TRIGGER takes SHARE ROW EXCLUSIVE on its table: writes to that table wait, reads do
-- not, until the migration commits. The rest take only ROW EXCLUSIVE. Production holds
-- tens of questions and a handful of accounts, so the transaction lasts milliseconds
-- to seconds; lock_timeout makes it fail at once, and be run again, rather than queue
-- every writer behind a long transaction. A failure rolls the whole transaction back,
-- the triggers included, so nothing is left disabled.
--
-- Idempotent: every statement selects only the rows that are not composed, so a
-- second run changes nothing. A value is left as it is if composing would take it past
-- the CHECK that bounds the column, which only a few scripts can do (U+0958 grows).
--
-- The deploy log says what it did, as NOTICEs: how many decomposed accepted answers
-- were folded into their composed twin, each column that still holds a row that is not
-- composed (only the CHECK guard leaves one), or that every listed column is composed.
SET LOCAL lock_timeout = '5s';

ALTER TABLE app.tests DISABLE TRIGGER tests_set_updated_at;
ALTER TABLE app.questions DISABLE TRIGGER questions_set_updated_at;
ALTER TABLE app.question_groups DISABLE TRIGGER question_groups_set_updated_at;
ALTER TABLE app.classes DISABLE TRIGGER classes_set_updated_at;
ALTER TABLE app.assignments DISABLE TRIGGER assignments_set_updated_at;
ALTER TABLE app.assignment_student_overrides DISABLE TRIGGER assignment_student_overrides_set_updated_at;
ALTER TABLE app.users DISABLE TRIGGER users_set_updated_at;
ALTER TABLE app.word_imports DISABLE TRIGGER word_imports_updated_at;

-- +goose StatementBegin
DO $$
DECLARE
  target    record;
  bound     text;
  only_rows text;
  folded    bigint;
  left_over bigint;
  total     bigint := 0;
BEGIN
  -- (blank_id, answer) is unique. Where a decomposed answer and its composed twin both
  -- exist, the grader already counts them as one, so keep the composed row and drop the other.
  DELETE FROM app.question_blank_answers
   WHERE id IN (SELECT id
                  FROM (SELECT id,
                               row_number() OVER (PARTITION BY blank_id, normalize(answer, NFC)
                                                  ORDER BY (answer IS NFC NORMALIZED) DESC, id) AS position
                          FROM app.question_blank_answers) ranked
                 WHERE position > 1);
  GET DIAGNOSTICS folded = ROW_COUNT;
  RAISE NOTICE 'F-37: folded % decomposed duplicate accepted answer(s) into their composed twin', folded;

  FOR target IN
    SELECT tbl, col, max_chars, only_where
      FROM (VALUES
        ('tests',                       'title',            200,    NULL),
        ('tests',                       'description',      NULL,   NULL),
        ('test_sections',               'title',            NULL,   NULL),
        ('test_sections',               'instructions',     NULL,   NULL),
        ('questions',                   'prompt',           NULL,   'prompt_content IS NULL'),
        ('questions',                   'explanation',      NULL,   'explanation_content IS NULL'),
        ('questions',                   'sample_answer',    NULL,   NULL),
        ('questions',                   'transcript',       NULL,   NULL),
        ('questions',                   'media_alt',        1000,   NULL),
        ('question_options',            'text',             NULL,   'content IS NULL'),
        ('question_blank_answers',      'answer',           NULL,   NULL),
        ('question_groups',             'title',            200,    NULL),
        ('group_stimuli',               'title',            200,    NULL),
        ('group_recordings',            'transcript',       100000, NULL),
        ('classes',                     'name',             120,    NULL),
        ('classes',                     'description',      NULL,   NULL),
        ('users',                       'full_name',        200,    NULL),
        ('users',                       'display_name',     80,     NULL),
        ('media_assets',                'display_name',     200,    NULL),
        ('media_assets',                'original_filename', NULL,  NULL),
        ('word_imports',                'title',            200,    NULL),
        ('word_import_sources',         'filename',         255,    NULL),
        ('assignments',                 'student_note',     500,    NULL),
        ('assignment_student_overrides', 'reason',          500,    NULL),
        ('attempts',                    'teacher_note',     2000,   NULL),
        ('attempts',                    'void_reason',      NULL,   NULL)
      ) AS targets(tbl, col, max_chars, only_where)
  LOOP
    bound := CASE WHEN target.max_chars IS NULL THEN ''
                  ELSE format(' AND char_length(normalize(%I, NFC)) <= %s', target.col, target.max_chars) END;
    only_rows := coalesce(target.only_where, 'true');
    EXECUTE format('UPDATE app.%1$I SET %2$I = normalize(%2$I, NFC) WHERE %2$I IS NOT NFC NORMALIZED%3$s AND (%4$s)',
                   target.tbl, target.col, bound, only_rows);
    EXECUTE format('SELECT count(*) FROM app.%1$I WHERE %2$I IS NOT NFC NORMALIZED AND (%3$s)',
                   target.tbl, target.col, only_rows) INTO left_over;
    IF left_over > 0 THEN
      total := total + left_over;
      RAISE NOTICE 'F-37: app.%.%: % row(s) left as they were, because composing would pass the limit of the column',
                   target.tbl, target.col, left_over;
    END IF;
  END LOOP;

  -- A tag is an element of an array: compose each, drop the repeat that composing makes
  -- of two spellings, and keep the order of first appearance.
  UPDATE app.questions q
     SET tags = ARRAY(SELECT composed.tag
                        FROM (SELECT normalize(spelling, NFC) AS tag, min(position) AS first_at
                                FROM unnest(q.tags) WITH ORDINALITY AS spellings(spelling, position)
                               GROUP BY 1) composed
                       ORDER BY composed.first_at)
   WHERE EXISTS (SELECT 1 FROM unnest(q.tags) AS spellings(spelling) WHERE spelling IS NOT NFC NORMALIZED);
  SELECT count(*) INTO left_over
    FROM app.questions q
   WHERE EXISTS (SELECT 1 FROM unnest(q.tags) AS spellings(spelling) WHERE spelling IS NOT NFC NORMALIZED);
  IF left_over > 0 THEN
    total := total + left_over;
    RAISE NOTICE 'F-37: app.questions.tags: % row(s) left as they were', left_over;
  END IF;

  IF total = 0 THEN
    RAISE NOTICE 'F-37: every listed column is composed';
  END IF;
END
$$;
-- +goose StatementEnd

ALTER TABLE app.tests ENABLE TRIGGER tests_set_updated_at;
ALTER TABLE app.questions ENABLE TRIGGER questions_set_updated_at;
ALTER TABLE app.question_groups ENABLE TRIGGER question_groups_set_updated_at;
ALTER TABLE app.classes ENABLE TRIGGER classes_set_updated_at;
ALTER TABLE app.assignments ENABLE TRIGGER assignments_set_updated_at;
ALTER TABLE app.assignment_student_overrides ENABLE TRIGGER assignment_student_overrides_set_updated_at;
ALTER TABLE app.users ENABLE TRIGGER users_set_updated_at;
ALTER TABLE app.word_imports ENABLE TRIGGER word_imports_updated_at;

-- +goose Down

-- Data only, and forward-safe: composed and decomposed text are the same text, and
-- nothing records which rows were decomposed, so there is nothing to restore.
-- The previous binary reads composed text exactly as it read decomposed text. The
-- deleted duplicate accepted answers were equal to the grader's eyes and are not
-- brought back.
SELECT 1;
