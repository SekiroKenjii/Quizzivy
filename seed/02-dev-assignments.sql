-- Development seed, part two: something for /admin to actually show.
--
-- Self-contained on purpose — it builds its own published test and version
-- rather than attaching to whatever happens to be in the database, so running
-- it twice is a no-op and running it on a fresh machine works.
--
-- Fixed uuids throughout, for the same reason.

-- ------------------------------------------------------------------ a test
INSERT INTO app.tests (id, title, description, status, current_version, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  'Unit 5 — Present perfect & listening',
  'Đề mẫu cho môi trường phát triển.',
  'published', 1,
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_versions (id, test_id, version, total_points, published_by)
VALUES (
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  1, '4.00',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_sections (id, test_version_id, ordinal, title)
VALUES (
  '01935000-0000-7000-8000-00000000dd03'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  0, 'Phần 1 · Ngữ pháp'
)
ON CONFLICT (id) DO NOTHING;

-- One auto-graded question and one that needs a human, so the grading queue on
-- /admin has something in it.
INSERT INTO app.test_version_questions
  (id, test_version_section_id, ordinal, type, prompt, points)
VALUES
  ('01935000-0000-7000-8000-00000000dd04'::uuid,
   '01935000-0000-7000-8000-00000000dd03'::uuid, 0,
   'single_choice', 'They ___ to the museum last weekend.', '2.00'),
  ('01935000-0000-7000-8000-00000000dd05'::uuid,
   '01935000-0000-7000-8000-00000000dd03'::uuid, 1,
   'short_answer', 'Viết 2–3 câu tả thói quen buổi sáng của bạn.', '2.00')
ON CONFLICT (id) DO NOTHING;

-- The choice question's options. publish would refuse a choice question
-- without a correct option, and the seeded attempt below answers with dd09, so
-- dd09 is the one that is right.
INSERT INTO app.test_version_options
  (id, test_version_question_id, ordinal, text, is_correct)
VALUES
  ('01935000-0000-7000-8000-00000000dd09'::uuid,
   '01935000-0000-7000-8000-00000000dd04'::uuid, 0, 'went', true),
  ('01935000-0000-7000-8000-00000000dd0a'::uuid,
   '01935000-0000-7000-8000-00000000dd04'::uuid, 1, 'go', false),
  ('01935000-0000-7000-8000-00000000dd0b'::uuid,
   '01935000-0000-7000-8000-00000000dd04'::uuid, 2, 'goes', false),
  ('01935000-0000-7000-8000-00000000dd0c'::uuid,
   '01935000-0000-7000-8000-00000000dd04'::uuid, 3, 'gone', false)
ON CONFLICT (id) DO NOTHING;

-- ------------------------------------------------------------ an assignment
-- Open right now, closing in three hours, so the dashboard's "Bài đang mở"
-- card and its countdown have something true to say.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000dd06'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  now() - interval '2 hours',
  now() + interval '3 hours',
  45,
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
VALUES (
  '01935000-0000-7000-8000-00000000dd06'::uuid,
  '01935000-0000-7000-8000-0000000000c1'::uuid
)
ON CONFLICT DO NOTHING;

-- --------------------------------------------------------------- an attempt
-- Submitted and flagged, with the short answer left ungraded: one row that
-- lights up three of the dashboard's four counters at once.
INSERT INTO app.attempts
  (id, assignment_id, test_version_id, student_id, attempt_no, status,
   session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at,
   submitted_at, flagged, focus_loss_count)
VALUES (
  '01935000-0000-7000-8000-00000000dd07'::uuid,
  '01935000-0000-7000-8000-00000000dd06'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  '01935000-0000-7000-8000-0000000000a2',
  1, 'submitted',
  '01935000-0000-7000-8000-00000000dd08'::uuid,
  20260829,
  sha256('dev-seed-beacon'::bytea),
  now() - interval '90 minutes',
  now() + interval '30 minutes',
  now() - interval '20 minutes',
  true, 4
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.attempt_answers (attempt_id, question_id, payload, requires_manual, auto_score)
VALUES
  ('01935000-0000-7000-8000-00000000dd07'::uuid,
   '01935000-0000-7000-8000-00000000dd04'::uuid,
   '{"optionIds":["01935000-0000-7000-8000-00000000dd09"]}'::jsonb, false, '2.00'),
  ('01935000-0000-7000-8000-00000000dd07'::uuid,
   '01935000-0000-7000-8000-00000000dd05'::uuid,
   '{"text":"I usually wake up at six."}'::jsonb, true, NULL)
ON CONFLICT DO NOTHING;
