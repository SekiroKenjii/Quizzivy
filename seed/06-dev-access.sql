-- Development seed, part six: staff other than the Admin (R2, T-R2.11).
--
-- Relies on nothing earlier: roles come from migration 00054, not a seed.
-- Every account's password is quizzivy-dev, and role_id is looked up by
-- builtin_key. Two Teachers own separate classes and tests, so the dev
-- database shows what one teacher cannot see of another's; the Assistant
-- exists so its narrower permissions can be tried by hand.
--
-- Fixed uuids. The rows a teacher can rewrite in the app (a draft's section
-- and its question, a group's member) are inserted only while their slot is
-- empty: the builder's autosave re-creates section rows under new ids, so a
-- conflict on the id alone would miss them and a second run would fail.

INSERT INTO app.users (id, email, full_name, role_id, password_hash, must_change_password)
VALUES
  ('01935000-0000-7000-8000-000000000601', 'giaovien@quizzivy.com', 'Trần Thị Lan',
   (SELECT id FROM app.roles WHERE builtin_key = 'teacher'),
   '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4', false),
  ('01935000-0000-7000-8000-000000000602', 'giaovien2@quizzivy.com', 'Lê Văn Minh',
   (SELECT id FROM app.roles WHERE builtin_key = 'teacher'),
   '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4', false),
  ('01935000-0000-7000-8000-000000000603', 'trogiang@quizzivy.com', 'Phạm Thu Hà',
   (SELECT id FROM app.roles WHERE builtin_key = 'assistant'),
   '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4', false)
ON CONFLICT DO NOTHING;

-- ------------------------------------------------------ giaovien's classroom

INSERT INTO app.classes (id, name, description, self_join_enabled, teacher_id)
VALUES (
  '01935000-0000-7000-8000-000000000611',
  'IELTS nền tảng — Lớp cô Lan',
  'Lớp mẫu của một giáo viên không phải quản trị viên.',
  true,
  '01935000-0000-7000-8000-000000000601'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.questions (id, type, prompt, points, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000000614',
  'short_answer', 'Describe your favourite place in your hometown.', '2.00',
  '01935000-0000-7000-8000-000000000601', '01935000-0000-7000-8000-000000000601'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.tests (id, title, description, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000000612',
  'Speaking warm-up (bản nháp)',
  'Đề nháp một câu hỏi từ ngân hàng của cô Lan.',
  '01935000-0000-7000-8000-000000000601', '01935000-0000-7000-8000-000000000601'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_sections (id, test_id, ordinal, title)
SELECT '01935000-0000-7000-8000-000000000613', '01935000-0000-7000-8000-000000000612', 0, 'Phần 1'
 WHERE NOT EXISTS (SELECT 1 FROM app.test_sections
                    WHERE id = '01935000-0000-7000-8000-000000000613' OR (test_id = '01935000-0000-7000-8000-000000000612' AND ordinal = 0));

INSERT INTO app.test_section_questions (id, test_section_id, ordinal, question_id)
SELECT '01935000-0000-7000-8000-000000000617', '01935000-0000-7000-8000-000000000613', 0, '01935000-0000-7000-8000-000000000614'
 WHERE EXISTS (SELECT 1 FROM app.test_sections WHERE id = '01935000-0000-7000-8000-000000000613')
   AND NOT EXISTS (SELECT 1 FROM app.test_section_questions WHERE test_section_id = '01935000-0000-7000-8000-000000000613')
   AND NOT EXISTS (SELECT 1 FROM app.test_section_units WHERE test_section_id = '01935000-0000-7000-8000-000000000613');

INSERT INTO app.question_groups (id, title, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000000615',
  'Đọc hiểu: A day at the market',
  '01935000-0000-7000-8000-000000000601', '01935000-0000-7000-8000-000000000601'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.questions
  (id, type, prompt, points, created_by, owner_id, context_group_id, context_ordinal, context_option_order)
SELECT '01935000-0000-7000-8000-000000000616',
       'short_answer', 'What did the writer buy first?', '1.00',
       '01935000-0000-7000-8000-000000000601', '01935000-0000-7000-8000-000000000601',
       '01935000-0000-7000-8000-000000000615', 0, 'shuffle'
 WHERE EXISTS (SELECT 1 FROM app.question_groups WHERE id = '01935000-0000-7000-8000-000000000615')
   AND NOT EXISTS (SELECT 1 FROM app.questions
                    WHERE id = '01935000-0000-7000-8000-000000000616'
                       OR (context_group_id = '01935000-0000-7000-8000-000000000615' AND context_ordinal = 0));

-- ----------------------------------------------------- giaovien2's classroom

INSERT INTO app.classes (id, name, description, self_join_enabled, teacher_id)
VALUES (
  '01935000-0000-7000-8000-000000000621',
  'Tiếng Anh thiếu nhi — Lớp thầy Minh',
  'Lớp của giáo viên thứ hai, để thử phạm vi dữ liệu.',
  true,
  '01935000-0000-7000-8000-000000000602'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.questions (id, type, prompt, points, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000000624',
  'short_answer', 'Write three animals that live on a farm.', '1.00',
  '01935000-0000-7000-8000-000000000602', '01935000-0000-7000-8000-000000000602'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.tests (id, title, description, created_by, owner_id)
VALUES (
  '01935000-0000-7000-8000-000000000622',
  'Farm animals quiz (bản nháp)',
  'Đề nháp của thầy Minh.',
  '01935000-0000-7000-8000-000000000602', '01935000-0000-7000-8000-000000000602'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.test_sections (id, test_id, ordinal, title)
SELECT '01935000-0000-7000-8000-000000000623', '01935000-0000-7000-8000-000000000622', 0, 'Phần 1'
 WHERE NOT EXISTS (SELECT 1 FROM app.test_sections
                    WHERE id = '01935000-0000-7000-8000-000000000623' OR (test_id = '01935000-0000-7000-8000-000000000622' AND ordinal = 0));

INSERT INTO app.test_section_questions (id, test_section_id, ordinal, question_id)
SELECT '01935000-0000-7000-8000-000000000627', '01935000-0000-7000-8000-000000000623', 0, '01935000-0000-7000-8000-000000000624'
 WHERE EXISTS (SELECT 1 FROM app.test_sections WHERE id = '01935000-0000-7000-8000-000000000623')
   AND NOT EXISTS (SELECT 1 FROM app.test_section_questions WHERE test_section_id = '01935000-0000-7000-8000-000000000623')
   AND NOT EXISTS (SELECT 1 FROM app.test_section_units WHERE test_section_id = '01935000-0000-7000-8000-000000000623');
