-- Development seed, part ten: a paper with one question of every type (R6, T-R6.14).
--
-- Relies on 01: the Admin (…00a1), who owns the paper, and the class (…00c1)
-- with hocvien@quizzivy.com in it. Nothing else here reads a later seed.
--
-- "Unit 7 — Every question type" is published in two parts:
--   Phần 1 · Đọc hiểu          a reading-passage group holding a True / False /
--                              Not given and a matching question, then a true / false
--   Phần 2 · Ngôn ngữ và viết  a single choice with an image in its prompt and in its
--                              explanation, a "Choose two" multiple choice, a fill in
--                              the blank, a short answer with a three-word limit, and
--                              an essay with four rubric criteria
--
-- The version is written the way publish writes it (delivery_version group_v1:
-- units, the group and its members, options, match items, blanks, rubric and
-- media references), and 99-assert.sql refuses a frozen row that breaks a
-- T-R6.4 publish rule, since this file skips publish's validation. The two
-- images are metadata only, like 08's (README.md, "Media objects"): the engine
-- and the result draw them with their alt text and no picture.
--
-- Three assignments to class …00c1, each open for a month with fifty attempts:
--   …1007  question-types.live.spec.ts: score, answers and explanations shown
--   …1008  E2E 9's session payload
--   …1009  E2E 9's result with score, answers and explanations hidden
--
-- Fixed uuids, …00000000 10NN, ON CONFLICT DO NOTHING like every other seed.

-- ------------------------------------------------------------------- media
INSERT INTO app.media_assets
  (id, kind, storage_key, mime_type, bytes, original_filename, checksum_sha256,
   uploaded_by, owner_id, display_name, width, height)
VALUES
  ('01935000-0000-7000-8000-000000001060'::uuid, 'image',
   'image/01935000-0000-7000-8000-000000001060.png', 'image/png', 98304,
   'Shop sign.png', sha256(convert_to('image/01935000-0000-7000-8000-000000001060.png', 'UTF8')),
   '01935000-0000-7000-8000-0000000000a1', '01935000-0000-7000-8000-0000000000a1',
   'Shop sign.png', 800, 600),
  ('01935000-0000-7000-8000-000000001061'::uuid, 'image',
   'image/01935000-0000-7000-8000-000000001061.png', 'image/png', 65536,
   'Opening hours.png', sha256(convert_to('image/01935000-0000-7000-8000-000000001061.png', 'UTF8')),
   '01935000-0000-7000-8000-0000000000a1', '01935000-0000-7000-8000-0000000000a1',
   'Opening hours.png', 800, 400)
ON CONFLICT (id) DO NOTHING;

-- -------------------------------------------------------------- the paper
INSERT INTO app.tests (id, title, description, status, current_version, last_published_version, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000001001'::uuid,
  'Unit 7 — Every question type',
  'Đề mẫu có đủ mọi loại câu hỏi của R6.',
  'published', 1, 1,
  '01935000-0000-7000-8000-0000000000a1',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

-- 1 + 3 + 1 in part 1, 1 + 2 + 2 + 1 + 9 in part 2.
INSERT INTO app.test_versions (id, test_id, version, total_points, published_by, delivery_version)
VALUES (
  '01935000-0000-7000-8000-000000001002'::uuid,
  '01935000-0000-7000-8000-000000001001'::uuid,
  1, '20.00',
  '01935000-0000-7000-8000-0000000000a1',
  'group_v1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_sections (id, test_version_id, ordinal, title, instructions)
VALUES
  ('01935000-0000-7000-8000-000000001003'::uuid,
   '01935000-0000-7000-8000-000000001002'::uuid, 0,
   'Phần 1 · Đọc hiểu', 'Đọc đoạn văn rồi trả lời.'),
  ('01935000-0000-7000-8000-000000001004'::uuid,
   '01935000-0000-7000-8000-000000001002'::uuid, 1,
   'Phần 2 · Ngôn ngữ và viết', NULL)
ON CONFLICT (id) DO NOTHING;

-- Ordinals run through each part in the order the student meets them.
INSERT INTO app.test_version_questions
  (id, test_version_section_id, ordinal, type, prompt, prompt_content, points,
   explanation, explanation_content, sample_answer, skill, select_count, max_words, match_allow_reuse)
VALUES
  ('01935000-0000-7000-8000-000000001011'::uuid,
   '01935000-0000-7000-8000-000000001003'::uuid, 0,
   'tfng', 'Most of the delta''s rice is grown in the dry season.', NULL, '1.00',
   'Đoạn văn nói lúa trồng nhiều nhất vào mùa mưa.', NULL, NULL, 'reading', NULL, NULL, NULL),
  ('01935000-0000-7000-8000-000000001012'::uuid,
   '01935000-0000-7000-8000-000000001003'::uuid, 1,
   'matching', 'Match each paragraph with its heading.', NULL, '3.00',
   NULL, NULL, NULL, 'reading', NULL, NULL, false),
  ('01935000-0000-7000-8000-000000001013'::uuid,
   '01935000-0000-7000-8000-000000001003'::uuid, 2,
   'true_false', 'The Mekong reaches the sea through nine mouths.', NULL, '1.00',
   NULL, NULL, NULL, 'reading', NULL, NULL, NULL),
  ('01935000-0000-7000-8000-000000001014'::uuid,
   '01935000-0000-7000-8000-000000001004'::uuid, 0,
   'single_choice',
   E'Look at the sign.\n\nA shop sign that reads: Closed on Sundays\n\nWhen is the shop closed?',
   '{"format":"semantic_v1","blocks":[
      {"type":"paragraph","content":[{"type":"text","text":"Look at the sign.","marks":[]}]},
      {"type":"image","assetId":"01935000-0000-7000-8000-000000001060","alt":"A shop sign that reads: Closed on Sundays"},
      {"type":"paragraph","content":[{"type":"text","text":"When is the shop closed?","marks":[]}]}]}'::jsonb,
   '1.00',
   E'"Closed on Sundays" means the shop does not open on that day.\n\nOpening hours: Monday to Saturday, 8:00 to 18:00',
   '{"format":"semantic_v1","blocks":[
      {"type":"paragraph","content":[{"type":"text","text":"\"Closed on Sundays\" means the shop does not open on that day.","marks":[]}]},
      {"type":"image","assetId":"01935000-0000-7000-8000-000000001061","alt":"Opening hours: Monday to Saturday, 8:00 to 18:00"}]}'::jsonb,
   NULL, 'vocabulary', NULL, NULL, NULL),
  ('01935000-0000-7000-8000-000000001015'::uuid,
   '01935000-0000-7000-8000-000000001004'::uuid, 1,
   'multiple_choice', 'Which TWO words are verbs?', NULL, '2.00',
   NULL, NULL, NULL, 'grammar', 2, NULL, NULL),
  ('01935000-0000-7000-8000-000000001016'::uuid,
   '01935000-0000-7000-8000-000000001004'::uuid, 2,
   'fill_blank', 'We {{1}} in Can Tho since {{2}}.', NULL, '2.00',
   NULL, NULL, NULL, 'grammar', NULL, NULL, NULL),
  ('01935000-0000-7000-8000-000000001017'::uuid,
   '01935000-0000-7000-8000-000000001004'::uuid, 3,
   'short_answer', 'What do farmers in the delta grow most?', NULL, '1.00',
   NULL, NULL, 'Rice', 'reading', NULL, 3, NULL),
  ('01935000-0000-7000-8000-000000001018'::uuid,
   '01935000-0000-7000-8000-000000001004'::uuid, 4,
   'essay',
   'Some people think rivers should be protected more than roads are built. Write at least 250 words.',
   NULL, '9.00',
   NULL, NULL, 'A balanced essay names both sides, gives an example of each and states a view.',
   'writing', NULL, NULL, NULL)
ON CONFLICT (id) DO NOTHING;

-- tfng: the three fixed texts in their fixed order, one correct. Matching:
-- choices i to iv, none marked correct; the items name the key.
INSERT INTO app.test_version_options
  (id, test_version_question_id, ordinal, text, is_correct)
VALUES
  ('01935000-0000-7000-8000-000000001020'::uuid, '01935000-0000-7000-8000-000000001011'::uuid, 0, 'True', false),
  ('01935000-0000-7000-8000-000000001021'::uuid, '01935000-0000-7000-8000-000000001011'::uuid, 1, 'False', true),
  ('01935000-0000-7000-8000-000000001022'::uuid, '01935000-0000-7000-8000-000000001011'::uuid, 2, 'Not given', false),
  ('01935000-0000-7000-8000-000000001023'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 0, 'Floods that feed the fields', false),
  ('01935000-0000-7000-8000-000000001024'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 1, 'A river with nine mouths', false),
  ('01935000-0000-7000-8000-000000001025'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 2, 'Markets on the water', false),
  ('01935000-0000-7000-8000-000000001026'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 3, 'Salt water moving inland', false),
  ('01935000-0000-7000-8000-000000001027'::uuid, '01935000-0000-7000-8000-000000001013'::uuid, 0, 'True', true),
  ('01935000-0000-7000-8000-000000001028'::uuid, '01935000-0000-7000-8000-000000001013'::uuid, 1, 'False', false),
  ('01935000-0000-7000-8000-000000001029'::uuid, '01935000-0000-7000-8000-000000001014'::uuid, 0, 'On Sundays', true),
  ('01935000-0000-7000-8000-00000000102a'::uuid, '01935000-0000-7000-8000-000000001014'::uuid, 1, 'On Saturdays', false),
  ('01935000-0000-7000-8000-00000000102b'::uuid, '01935000-0000-7000-8000-000000001014'::uuid, 2, 'Every evening', false),
  ('01935000-0000-7000-8000-00000000102c'::uuid, '01935000-0000-7000-8000-000000001015'::uuid, 0, 'harvest', true),
  ('01935000-0000-7000-8000-00000000102d'::uuid, '01935000-0000-7000-8000-000000001015'::uuid, 1, 'flood', true),
  ('01935000-0000-7000-8000-00000000102e'::uuid, '01935000-0000-7000-8000-000000001015'::uuid, 2, 'delta', false),
  ('01935000-0000-7000-8000-00000000102f'::uuid, '01935000-0000-7000-8000-000000001015'::uuid, 3, 'muddy', false),
  ('01935000-0000-7000-8000-000000001030'::uuid, '01935000-0000-7000-8000-000000001015'::uuid, 4, 'slowly', false)
ON CONFLICT (id) DO NOTHING;

-- A is ii, B is iv, C is i; iii is the extra choice.
INSERT INTO app.test_version_match_items (id, test_version_question_id, ordinal, text, correct_option_id)
VALUES
  ('01935000-0000-7000-8000-000000001040'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 0,
   'Paragraph A', '01935000-0000-7000-8000-000000001024'::uuid),
  ('01935000-0000-7000-8000-000000001041'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 1,
   'Paragraph B', '01935000-0000-7000-8000-000000001026'::uuid),
  ('01935000-0000-7000-8000-000000001042'::uuid, '01935000-0000-7000-8000-000000001012'::uuid, 2,
   'Paragraph C', '01935000-0000-7000-8000-000000001023'::uuid)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_blanks (id, test_version_question_id, ordinal, case_sensitive)
VALUES
  ('01935000-0000-7000-8000-000000001048'::uuid, '01935000-0000-7000-8000-000000001016'::uuid, 1, false),
  ('01935000-0000-7000-8000-000000001049'::uuid, '01935000-0000-7000-8000-000000001016'::uuid, 2, false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_blank_answers (id, test_version_blank_id, answer)
VALUES
  ('01935000-0000-7000-8000-00000000104a'::uuid, '01935000-0000-7000-8000-000000001048'::uuid, 'have lived'),
  ('01935000-0000-7000-8000-00000000104b'::uuid, '01935000-0000-7000-8000-000000001048'::uuid, '''ve lived'),
  ('01935000-0000-7000-8000-00000000104c'::uuid, '01935000-0000-7000-8000-000000001049'::uuid, '2019')
ON CONFLICT (id) DO NOTHING;

-- The "IELTS Writing" preset's four labels.
INSERT INTO app.test_version_rubric_criteria (id, test_version_question_id, ordinal, label)
VALUES
  ('01935000-0000-7000-8000-000000001050'::uuid, '01935000-0000-7000-8000-000000001018'::uuid, 0, 'Task response'),
  ('01935000-0000-7000-8000-000000001051'::uuid, '01935000-0000-7000-8000-000000001018'::uuid, 1, 'Coherence and cohesion'),
  ('01935000-0000-7000-8000-000000001052'::uuid, '01935000-0000-7000-8000-000000001018'::uuid, 2, 'Lexical resource'),
  ('01935000-0000-7000-8000-000000001053'::uuid, '01935000-0000-7000-8000-000000001018'::uuid, 3, 'Grammatical range and accuracy')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_question_media_references
  (test_version_question_id, field, media_asset_id, media_asset_kind)
VALUES
  ('01935000-0000-7000-8000-000000001014'::uuid, 'prompt', '01935000-0000-7000-8000-000000001060'::uuid, 'image'),
  ('01935000-0000-7000-8000-000000001014'::uuid, 'explanation', '01935000-0000-7000-8000-000000001061'::uuid, 'image')
ON CONFLICT DO NOTHING;

-- ----------------------------------------------------- the reading group
-- tfng and matching members keep their options in order ("fixed"), as the
-- builder writes them.
INSERT INTO app.test_version_groups (id, test_version_section_id, title, instructions)
VALUES (
  '01935000-0000-7000-8000-000000001005'::uuid,
  '01935000-0000-7000-8000-000000001003'::uuid,
  'Reading passage · The Mekong Delta',
  '{"format":"semantic_v1","blocks":[{"type":"paragraph","content":[{"type":"text","text":"Read the passage, then answer questions 1 and 2.","marks":[]}]}]}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_group_stimuli (id, group_id, ordinal, title, content)
VALUES (
  '01935000-0000-7000-8000-000000001006'::uuid,
  '01935000-0000-7000-8000-000000001005'::uuid, 0,
  'The Mekong Delta',
  '{"format":"semantic_v1","blocks":[
     {"type":"paragraph","content":[{"type":"text","text":"A. The Mekong ends its long journey in southern Vietnam, where it splits into many branches. People call it the river of nine dragons.","marks":[]}]},
     {"type":"paragraph","content":[{"type":"text","text":"B. Each dry season the sea pushes salt water further up these branches, and some farmers now grow shrimp where rice once stood.","marks":[]}]},
     {"type":"paragraph","content":[{"type":"text","text":"C. In the rainy season the river floods the plains and leaves rich silt behind. Most of the delta''s rice is planted then.","marks":[]}]}]}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_version_group_members (group_id, test_version_section_id, question_id, ordinal, option_order)
VALUES
  ('01935000-0000-7000-8000-000000001005'::uuid, '01935000-0000-7000-8000-000000001003'::uuid,
   '01935000-0000-7000-8000-000000001011'::uuid, 0, 'fixed'),
  ('01935000-0000-7000-8000-000000001005'::uuid, '01935000-0000-7000-8000-000000001003'::uuid,
   '01935000-0000-7000-8000-000000001012'::uuid, 1, 'fixed')
ON CONFLICT DO NOTHING;

INSERT INTO app.test_version_units (id, test_version_section_id, ordinal, question_id, group_id)
VALUES
  ('01935000-0000-7000-8000-00000000100a'::uuid, '01935000-0000-7000-8000-000000001003'::uuid, 0,
   NULL, '01935000-0000-7000-8000-000000001005'::uuid),
  ('01935000-0000-7000-8000-00000000100b'::uuid, '01935000-0000-7000-8000-000000001003'::uuid, 1,
   '01935000-0000-7000-8000-000000001013'::uuid, NULL),
  ('01935000-0000-7000-8000-00000000100c'::uuid, '01935000-0000-7000-8000-000000001004'::uuid, 0,
   '01935000-0000-7000-8000-000000001014'::uuid, NULL),
  ('01935000-0000-7000-8000-00000000100d'::uuid, '01935000-0000-7000-8000-000000001004'::uuid, 1,
   '01935000-0000-7000-8000-000000001015'::uuid, NULL),
  ('01935000-0000-7000-8000-00000000100e'::uuid, '01935000-0000-7000-8000-000000001004'::uuid, 2,
   '01935000-0000-7000-8000-000000001016'::uuid, NULL),
  ('01935000-0000-7000-8000-00000000100f'::uuid, '01935000-0000-7000-8000-000000001004'::uuid, 3,
   '01935000-0000-7000-8000-000000001017'::uuid, NULL),
  ('01935000-0000-7000-8000-000000001010'::uuid, '01935000-0000-7000-8000-000000001004'::uuid, 4,
   '01935000-0000-7000-8000-000000001018'::uuid, NULL)
ON CONFLICT (id) DO NOTHING;

-- ------------------------------------------------------------- assignments
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes, max_attempts,
   review_show_score, review_show_correct_answers, review_show_explanations,
   published_at, created_by)
SELECT a.id, '01935000-0000-7000-8000-000000001001'::uuid, '01935000-0000-7000-8000-000000001002'::uuid,
       now() - interval '1 hour', now() + interval '30 days', 60, 50,
       a.shown, a.shown, a.shown,
       now() - interval '1 hour', '01935000-0000-7000-8000-0000000000a1'::uuid
  FROM (VALUES
    ('01935000-0000-7000-8000-000000001007'::uuid, true),
    ('01935000-0000-7000-8000-000000001008'::uuid, true),
    ('01935000-0000-7000-8000-000000001009'::uuid, false)
  ) AS a(id, shown)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
SELECT a, '01935000-0000-7000-8000-0000000000c1'::uuid
  FROM unnest(ARRAY[
    '01935000-0000-7000-8000-000000001007'::uuid,
    '01935000-0000-7000-8000-000000001008'::uuid,
    '01935000-0000-7000-8000-000000001009'::uuid
  ]) AS a
ON CONFLICT DO NOTHING;
