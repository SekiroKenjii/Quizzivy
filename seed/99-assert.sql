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

-- 08's teacher workspace (T-R4.51): the rows exist in the shapes the Teacher
-- deck's fixtures need. Every id here starts 01935000-0000-7000-8000-00000080.
DO $$
DECLARE
  lan constant uuid := '01935000-0000-7000-8000-000000000601';
  mine constant text := '01935000-0000-7000-8000-00000080%';
  showcase uuid;
  bad text;
  found int;
BEGIN
  SELECT count(*) INTO found FROM app.tests WHERE id::text LIKE mine AND owner_id = lan AND created_by = lan;
  IF found <> 7 THEN
    RAISE EXCEPTION 'seed: expected seven tests owned by the teacher, found %', found;
  END IF;

  -- One test with three versions and a draft ahead of the third: a bank
  -- question in the draft that the third version does not hold, and a prompt
  -- the bank has since corrected.
  SELECT count(*) INTO found
    FROM app.tests t
   WHERE t.id::text LIKE mine AND t.status = 'published' AND t.current_version = 3 AND t.last_published_version = 3
     AND (SELECT count(*) FROM app.test_versions v WHERE v.test_id = t.id) = 3
     AND EXISTS (SELECT 1 FROM app.test_sections s JOIN app.test_section_questions m ON m.test_section_id = s.id
                  WHERE s.test_id = t.id
                    AND NOT EXISTS (SELECT 1 FROM app.test_version_questions f
                                      JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id
                                      JOIN app.test_versions v ON v.id = vs.test_version_id AND v.test_id = t.id AND v.version = 3
                                     WHERE f.source_question_id = m.question_id))
     AND EXISTS (SELECT 1 FROM app.test_version_questions f
                   JOIN app.questions q ON q.id = f.source_question_id
                   JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id
                   JOIN app.test_versions v ON v.id = vs.test_version_id AND v.test_id = t.id AND v.version = 3
                  WHERE f.prompt <> q.prompt);
  IF found <> 1 THEN
    RAISE EXCEPTION 'seed: expected one published test with three versions and a draft ahead, found %', found;
  END IF;

  -- A version is written as publish writes it.
  SELECT string_agg(v.id::text, ', ') INTO bad
    FROM app.test_versions v
   WHERE v.test_id::text LIKE mine
     AND (v.delivery_version <> 'group_v1'
          OR v.total_points IS DISTINCT FROM (SELECT sum(f.points)
                                                FROM app.test_version_questions f
                                                JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id
                                               WHERE vs.test_version_id = v.id));
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: version(s) not written as publish writes them: %', bad;
  END IF;

  SELECT count(*) INTO found FROM app.media_assets
   WHERE id::text LIKE mine AND owner_id = lan AND uploaded_by = lan AND deleted_at IS NULL;
  IF found <> 8 THEN
    RAISE EXCEPTION 'seed: expected eight media files owned by the teacher, found %', found;
  END IF;

  SELECT string_agg(c.name || '=' || c.members, ', ') INTO bad
    FROM (SELECT c.name, (SELECT count(*) FROM app.class_members m WHERE m.class_id = c.id) AS members
            FROM app.classes c WHERE c.id::text LIKE mine AND c.teacher_id = lan) c
   WHERE (c.name, c.members) NOT IN (('IELTS 6.5 Evening', 24), ('IELTS Foundation A', 16), ('TOEIC 600 Weekend', 22), ('Kids Starters B', 12));
  SELECT count(*) INTO found FROM app.classes WHERE id::text LIKE mine AND teacher_id = lan;
  IF bad IS NOT NULL OR found <> 4 THEN
    RAISE EXCEPTION 'seed: expected four classes with the deck''s members, found % (off: %)', found, bad;
  END IF;

  SELECT count(DISTINCT CASE
           WHEN a.published_at IS NULL THEN 'draft'
           WHEN a.closed_at IS NOT NULL AND now() >= a.closed_at THEN 'closed'
           WHEN now() < a.opens_at THEN 'scheduled'
           WHEN now() < a.closes_at THEN 'open'
           ELSE 'closed' END) INTO found
    FROM app.assignments a WHERE a.id::text LIKE mine AND a.created_by = lan;
  IF found <> 4 THEN
    RAISE EXCEPTION 'seed: expected assignments in all four statuses, found % statuses', found;
  END IF;

  SELECT count(*) INTO found FROM app.attempts a
   WHERE a.assignment_id::text LIKE mine AND a.flagged AND a.focus_loss_count > 0
     AND EXISTS (SELECT 1 FROM app.attempt_events e WHERE e.attempt_id = a.id AND e.kind = 'fullscreen_exit');
  IF found < 1 THEN
    RAISE EXCEPTION 'seed: no flagged attempt with its integrity events';
  END IF;

  SELECT count(DISTINCT a.id) INTO found FROM app.attempts a
    JOIN app.attempt_answers aa ON aa.attempt_id = a.id
   WHERE a.assignment_id::text LIKE mine AND a.status IN ('submitted', 'timed_out')
     AND aa.requires_manual AND aa.manual_score IS NULL;
  IF found <> 14 THEN
    RAISE EXCEPTION 'seed: expected fourteen attempts waiting to be graded, found %', found;
  END IF;

  SELECT string_agg(a.id::text, ', ') INTO bad FROM app.attempts a
   WHERE a.assignment_id::text LIKE mine AND a.status IN ('submitted', 'graded', 'timed_out')
     AND a.score_earned IS DISTINCT FROM (SELECT coalesce(sum(aa.final_score), 0) FROM app.attempt_answers aa WHERE aa.attempt_id = a.id);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: attempt(s) whose score is not the sum of their answers: %', bad;
  END IF;

  -- Four imports in four states; the pasted one is in review with two open
  -- actions and one thing to confirm.
  SELECT string_agg(i.status, ', ' ORDER BY i.status) INTO bad
    FROM app.word_imports i WHERE i.id::text LIKE mine AND i.created_by = lan;
  IF bad IS DISTINCT FROM 'committed, failed, needs_review, processing' THEN
    RAISE EXCEPTION 'seed: expected four imports in four states, found %', bad;
  END IF;
  SELECT count(*) INTO found
    FROM app.word_imports i
    JOIN app.word_import_sources s ON s.import_id = i.id AND s.format = 'text' AND s.characters > 0
    JOIN app.word_import_drafts d ON d.import_id = i.id
   WHERE i.id::text LIKE mine AND i.status = 'needs_review' AND d.open_action_count = 2 AND d.open_confirm_count = 1;
  IF found <> 1 THEN
    RAISE EXCEPTION 'seed: no pasted-text import in review with two open actions and one to confirm';
  END IF;
  SELECT count(*) INTO found
    FROM app.word_import_commits c JOIN app.tests t ON t.id = c.test_id
   WHERE c.import_id::text LIKE mine AND t.owner_id = lan AND t.status = 'draft';
  IF found <> 1 THEN
    RAISE EXCEPTION 'seed: the committed import does not point at a draft test';
  END IF;

  -- One test holds a question of each content kind, in the bank and in its
  -- published version.
  SELECT t.id INTO showcase FROM app.tests t WHERE t.id::text LIKE mine AND t.title LIKE 'Unit 4%';
  SELECT string_agg(kind.name, ', ') INTO bad
    FROM (VALUES
      ('table and link'), ('two gaps'), ('markdown'), ('audio'), ('image with alt')
    ) AS kind(name)
   WHERE NOT EXISTS (
     SELECT 1
       FROM app.test_sections s
       JOIN app.test_section_questions m ON m.test_section_id = s.id
       JOIN app.questions q ON q.id = m.question_id
      WHERE s.test_id = showcase
        AND CASE kind.name
              WHEN 'table and link' THEN jsonb_path_exists(q.prompt_content, '$.blocks[*] ? (@.type == "table")')
                                     AND jsonb_path_exists(q.prompt_content, '$.** ? (@.type == "link")')
              WHEN 'two gaps' THEN q.type = 'fill_blank' AND q.prompt_content IS NOT NULL
                               AND (SELECT count(*) FROM app.question_blanks b WHERE b.question_id = q.id AND b.gap_id IS NOT NULL) = 2
              WHEN 'markdown' THEN q.prompt_content IS NULL AND q.prompt LIKE '%**%'
              WHEN 'audio' THEN q.media_asset_kind = 'audio' AND q.transcript IS NOT NULL
              ELSE q.media_asset_kind = 'image' AND q.media_alt IS NOT NULL END);
  IF bad IS NOT NULL THEN
    RAISE EXCEPTION 'seed: the content-kind test lacks: %', bad;
  END IF;
  SELECT count(*) INTO found
    FROM app.test_version_questions f
    JOIN app.test_version_sections vs ON vs.id = f.test_version_section_id
    JOIN app.test_versions v ON v.id = vs.test_version_id
   WHERE v.test_id = showcase AND (f.prompt_content IS NOT NULL OR f.media_alt IS NOT NULL OR f.media_asset_kind = 'audio');
  IF found < 4 THEN
    RAISE EXCEPTION 'seed: the content-kind test''s version froze fewer rich questions than the bank holds: %', found;
  END IF;

  RAISE NOTICE 'seed: teacher workspace holds';
END $$;
