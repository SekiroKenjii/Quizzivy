-- Development seed, part five: a paper with more than one part.
--
-- 02's test has a single section, which is the common case and the one
-- every E2E fixture shares. The engine's navigator groups by section and a
-- section's instructions are shown above its first question (S-05, S-06,
-- S-08), so the dev database needs one paper where that is visible: three
-- parts, two with instructions, four question types, no audio (E2E 8 authors
-- its own audio test through the admin UI, for the same reason as before).
--
-- Fixed uuids, ON CONFLICT DO NOTHING, like every other seed file.

INSERT INTO app.tests (id, title, description, status, current_version, created_by)
VALUES (
  '01935000-0000-7000-8000-000000005e01'::uuid,
  'Unit 6 — Reported speech & writing',
  'Đề mẫu ba phần cho môi trường phát triển.',
  'published', 1,
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_versions (id, test_id, version, total_points, published_by)
VALUES (
  '01935000-0000-7000-8000-000000005e02'::uuid,
  '01935000-0000-7000-8000-000000005e01'::uuid,
  1, '9.00',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_sections (id, test_version_id, ordinal, title, instructions)
VALUES
  ('01935000-0000-7000-8000-000000005e03'::uuid,
   '01935000-0000-7000-8000-000000005e02'::uuid, 0,
   'Phần 1 · Ngữ pháp', NULL),
  ('01935000-0000-7000-8000-000000005e04'::uuid,
   '01935000-0000-7000-8000-000000005e02'::uuid, 1,
   'Phần 2 · Điền từ', 'Điền từ thích hợp vào chỗ trống. Mỗi chỗ trống một hoặc hai từ.'),
  ('01935000-0000-7000-8000-000000005e05'::uuid,
   '01935000-0000-7000-8000-000000005e02'::uuid, 2,
   'Phần 3 · Viết', 'Viết 2–3 câu, dùng câu tường thuật.')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_questions
  (id, test_version_section_id, ordinal, type, prompt, points, explanation)
VALUES
  ('01935000-0000-7000-8000-000000005e10'::uuid,
   '01935000-0000-7000-8000-000000005e03'::uuid, 0,
   'single_choice', 'She said she ___ tired.', '2.00',
   'Lùi một thì: "am/is" trong lời nói trực tiếp thành "was" khi tường thuật.'),
  ('01935000-0000-7000-8000-000000005e11'::uuid,
   '01935000-0000-7000-8000-000000005e03'::uuid, 1,
   'multiple_choice', 'Which of these are reporting verbs?', '2.00', NULL),
  ('01935000-0000-7000-8000-000000005e12'::uuid,
   '01935000-0000-7000-8000-000000005e04'::uuid, 0,
   'fill_blank', 'He told me that he {{1}} the film the day {{2}}.', '2.00', NULL),
  ('01935000-0000-7000-8000-000000005e13'::uuid,
   '01935000-0000-7000-8000-000000005e05'::uuid, 0,
   'short_answer', 'Report what a friend told you yesterday, in two or three sentences.', '3.00', NULL)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_options
  (id, test_version_question_id, ordinal, text, is_correct)
VALUES
  ('01935000-0000-7000-8000-000000005e20'::uuid,
   '01935000-0000-7000-8000-000000005e10'::uuid, 0, 'was', true),
  ('01935000-0000-7000-8000-000000005e21'::uuid,
   '01935000-0000-7000-8000-000000005e10'::uuid, 1, 'is', false),
  ('01935000-0000-7000-8000-000000005e22'::uuid,
   '01935000-0000-7000-8000-000000005e10'::uuid, 2, 'were', false),
  ('01935000-0000-7000-8000-000000005e23'::uuid,
   '01935000-0000-7000-8000-000000005e10'::uuid, 3, 'be', false),
  ('01935000-0000-7000-8000-000000005e24'::uuid,
   '01935000-0000-7000-8000-000000005e11'::uuid, 0, 'said', true),
  ('01935000-0000-7000-8000-000000005e25'::uuid,
   '01935000-0000-7000-8000-000000005e11'::uuid, 1, 'told', true),
  ('01935000-0000-7000-8000-000000005e26'::uuid,
   '01935000-0000-7000-8000-000000005e11'::uuid, 2, 'table', false),
  ('01935000-0000-7000-8000-000000005e27'::uuid,
   '01935000-0000-7000-8000-000000005e11'::uuid, 3, 'green', false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_blanks (id, test_version_question_id, ordinal, case_sensitive)
VALUES
  ('01935000-0000-7000-8000-000000005e30'::uuid,
   '01935000-0000-7000-8000-000000005e12'::uuid, 1, false),
  ('01935000-0000-7000-8000-000000005e31'::uuid,
   '01935000-0000-7000-8000-000000005e12'::uuid, 2, false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_blank_answers (id, test_version_blank_id, answer)
VALUES
  ('01935000-0000-7000-8000-000000005e40'::uuid,
   '01935000-0000-7000-8000-000000005e30'::uuid, 'had seen'),
  ('01935000-0000-7000-8000-000000005e41'::uuid,
   '01935000-0000-7000-8000-000000005e31'::uuid, 'before')
ON CONFLICT (id) DO NOTHING;

-- Open for a month with generous attempts: a developer opens this paper again
-- and again, and "hết lượt" is not what is being looked at.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes,
   max_attempts, published_at, created_by)
VALUES (
  '01935000-0000-7000-8000-000000005e06'::uuid,
  '01935000-0000-7000-8000-000000005e01'::uuid,
  '01935000-0000-7000-8000-000000005e02'::uuid,
  now() - interval '1 hour',
  now() + interval '30 days',
  45,
  50,
  now() - interval '1 hour',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
VALUES (
  '01935000-0000-7000-8000-000000005e06'::uuid,
  '01935000-0000-7000-8000-0000000000c1'::uuid
)
ON CONFLICT DO NOTHING;
