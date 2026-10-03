-- Seed self-check. Runs last.
--
-- The seed writes frozen version rows directly, which is the one path that
-- skips publish's validation (internal/tests/publish/validate.go) -- and so
-- the one path that can produce data the product cannot. A choice question
-- with no options renders as a prompt and nothing else, and every answer a
-- client builds for it is rejected as invalid. Refuse the seed instead.
DO $$
DECLARE
  bad text;
BEGIN
  -- Bank: choice questions carry at least two options and a correct one.
  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.questions q
   WHERE q.type IN ('single_choice', 'multiple_choice')
     AND q.deleted_at IS NULL
     AND (SELECT count(*) FROM app.question_options o WHERE o.question_id = q.id) < 2;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: bank choice question(s) with fewer than two options: %', bad;
  END IF;

  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.questions q
   WHERE q.type IN ('single_choice', 'multiple_choice')
     AND q.deleted_at IS NULL
     AND NOT EXISTS (SELECT 1 FROM app.question_options o
                      WHERE o.question_id = q.id AND o.is_correct);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: bank choice question(s) with no correct option: %', bad;
  END IF;

  -- Frozen: the same rules, on the rows a student actually answers.
  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.test_version_questions q
   WHERE q.type IN ('single_choice', 'multiple_choice')
     AND (SELECT count(*) FROM app.test_version_options o
           WHERE o.test_version_question_id = q.id) < 2;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: frozen choice question(s) with fewer than two options: %', bad;
  END IF;

  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.test_version_questions q
   WHERE q.type IN ('single_choice', 'multiple_choice')
     AND NOT EXISTS (SELECT 1 FROM app.test_version_options o
                      WHERE o.test_version_question_id = q.id AND o.is_correct);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: frozen choice question(s) with no correct option: %', bad;
  END IF;

  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.test_version_questions q
   WHERE q.type = 'single_choice'
     AND (SELECT count(*) FROM app.test_version_options o
           WHERE o.test_version_question_id = q.id AND o.is_correct) > 1;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: frozen single_choice question(s) with more than one correct option: %', bad;
  END IF;

  SELECT string_agg(q.id::text, ', ') INTO bad
    FROM app.test_version_questions q
   WHERE q.type = 'fill_blank'
     AND NOT EXISTS (SELECT 1 FROM app.test_version_blanks b
                      WHERE b.test_version_question_id = q.id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: frozen fill_blank question(s) with no blanks: %', bad;
  END IF;

  -- A seeded answer must point at options that exist on its own question.
  -- Seeded rows only: every seed id starts with 01935000-, and what students
  -- and tests wrote into a dev database since is not the seed's to vouch for.
  SELECT string_agg(a.attempt_id::text || '/' || a.question_id::text, ', ') INTO bad
    FROM app.attempt_answers a
   WHERE a.attempt_id::text LIKE '01935000-%'
     AND jsonb_typeof(a.payload->'optionIds') = 'array'
     AND EXISTS (
       SELECT 1 FROM jsonb_array_elements_text(a.payload->'optionIds') chosen(id)
        WHERE NOT EXISTS (SELECT 1 FROM app.test_version_options o
                           WHERE o.id::text = chosen.id
                             AND o.test_version_question_id = a.question_id));
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: attempt answer(s) choosing options that do not exist: %', bad;
  END IF;

  -- R2's access model: the built-in roles exist, and every row it scopes by
  -- has the column it reads (T-R2.11).
  IF (SELECT count(*) FROM app.roles
       WHERE builtin_key IN ('admin', 'teacher', 'assistant', 'student')) <> 4 THEN
    RAISE EXCEPTION 'seed: the four built-in roles are not all present';
  END IF;

  SELECT string_agg(email, ', ') INTO bad FROM app.users WHERE role_id IS NULL;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: user(s) without role_id: %', bad;
  END IF;

  SELECT string_agg(id::text, ', ') INTO bad FROM app.classes WHERE teacher_id IS NULL;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: class(es) without teacher_id: %', bad;
  END IF;

  SELECT string_agg(owned.kind || ' ' || owned.id::text, ', ') INTO bad
    FROM (SELECT 'test' AS kind, id FROM app.tests WHERE owner_id IS NULL
          UNION ALL SELECT 'question', id FROM app.questions WHERE owner_id IS NULL
          UNION ALL SELECT 'question group', id FROM app.question_groups WHERE owner_id IS NULL
          UNION ALL SELECT 'media asset', id FROM app.media_assets WHERE owner_id IS NULL) owned;
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: row(s) without owner_id: %', bad;
  END IF;

  RAISE NOTICE 'seed: invariants hold';
END $$;
