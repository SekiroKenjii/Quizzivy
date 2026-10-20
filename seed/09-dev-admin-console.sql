-- Development seed, part nine: the Admin deck's fixtures (R5, T-R5.33).
--
-- Relies on 01 (the Admin, thuong@quizzivy.com), 03 (students), 06 (two
-- Teachers and the Assistant) and 08 (the teacher workspace: its classes,
-- test, media, imports and roster). Roles come from migrations, not a seed.
--
-- Brings the console to the deck's shape: three Admins, nine Teachers (one
-- disabled, teaching the archived class), students in every status, the
-- deck's two extra classes and its archived one, a custom role "Tổ trưởng",
-- audit entries in every category plus one written by the system,
-- fourteen days of activity, and this month's AI credits with one request
-- waiting. Every account's password is quizzivy-dev.
--
-- Times are relative to now(), so "last active" and the activity chart read
-- the same whenever the seed runs. Ids are fixed (…0000009 + a number). The
-- audit entries are inserted once, while none from this file exist, since
-- audit_log has no natural key to conflict on; a later run adds only the
-- activity days since the last one.

INSERT INTO app.users
  (id, email, full_name, role_id, password_hash, must_change_password, disabled_at,
   last_sign_in_at, last_sign_in_method, last_active_at, created_by)
SELECT v.id::uuid, v.email, v.full_name,
       (SELECT id FROM app.roles WHERE builtin_key = v.role),
       CASE WHEN v.pw THEN '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4' END,
       v.must_change,
       CASE WHEN v.disabled_ago IS NOT NULL THEN now() - v.disabled_ago::interval END,
       CASE WHEN v.seen_ago IS NOT NULL THEN now() - v.seen_ago::interval END,
       CASE WHEN v.seen_ago IS NOT NULL THEN v.method END,
       CASE WHEN v.seen_ago IS NOT NULL THEN now() - v.seen_ago::interval END,
       '01935000-0000-7000-8000-0000000000a1'::uuid
  FROM (VALUES
    ('01935000-0000-7000-8000-000000900001', 'yen.nguyen@quizzivy.com',   'Nguyễn Hải Yến',  'admin',   true,  false, NULL,      '2 hours',  'password'),
    ('01935000-0000-7000-8000-000000900002', 'quan.do@quizzivy.com',      'Đỗ Minh Quân',    'admin',   true,  false, NULL,      '3 days',   'password'),
    ('01935000-0000-7000-8000-000000900011', 'giau.bui@quizzivy.com',     'Giàu Bùi',        'teacher', true,  false, NULL,      '5 minutes','password'),
    ('01935000-0000-7000-8000-000000900012', 'duc.phan@quizzivy.com',     'Phan Minh Đức',   'teacher', false, false, NULL,      '1 day',    'google'),
    ('01935000-0000-7000-8000-000000900013', 'van.ly@quizzivy.com',       'Lý Thanh Vân',    'teacher', true,  false, '27 days', '14 days',  'password'),
    ('01935000-0000-7000-8000-000000900014', 'bao.dinh@quizzivy.com',     'Đinh Quốc Bảo',   'teacher', true,  false, NULL,      '4 hours',  'password'),
    ('01935000-0000-7000-8000-000000900015', 'trang.ngo@quizzivy.com',    'Ngô Thu Trang',   'teacher', true,  false, NULL,      '2 days',   'password'),
    ('01935000-0000-7000-8000-000000900016', 'khai.truong@quizzivy.com',  'Trương Văn Khải', 'teacher', true,  true,  NULL,      '6 days',   'password'),
    ('01935000-0000-7000-8000-000000900017', 'linh.ha@quizzivy.com',      'Hà Mỹ Linh',      'teacher', true,  false, NULL,      NULL,       NULL),
    ('01935000-0000-7000-8000-000000900021', 'bich.hoang@quizzivy.com',   'Hoàng Ngọc Bích', 'student', true,  false, '9 days',  '20 days',  'password'),
    ('01935000-0000-7000-8000-000000900022', 'son.vu@quizzivy.com',       'Vũ Thanh Sơn',    'student', true,  true,  NULL,      '3 days',   'password'),
    ('01935000-0000-7000-8000-000000900023', 'tam.truong@quizzivy.com',   'Trương Minh Tâm', 'student', true,  false, NULL,      NULL,       NULL),
    ('01935000-0000-7000-8000-000000900024', 'an.lam@quizzivy.com',       'Lâm Bảo An',      'student', true,  false, NULL,      '40 minutes','password'),
    ('01935000-0000-7000-8000-000000900025', 'chi.mai@quizzivy.com',      'Mai Linh Chi',    'student', false, false, NULL,      '1 hour',   'google'),
    ('01935000-0000-7000-8000-000000900026', 'khoa.dinh@quizzivy.com',    'Đinh Đăng Khoa',  'student', true,  false, NULL,      '5 hours',  'password'),
    ('01935000-0000-7000-8000-000000900027', 'nhi.ta@quizzivy.com',       'Tạ Yến Nhi',      'student', true,  false, NULL,      '2 days',   'password'),
    ('01935000-0000-7000-8000-000000900028', 'phuc.luu@quizzivy.com',     'Lưu Hoàng Phúc',  'student', true,  false, NULL,      '8 days',   'password')
  ) AS v(id, email, full_name, role, pw, must_change, disabled_ago, seen_ago, method)
ON CONFLICT DO NOTHING;

INSERT INTO app.user_identities (id, user_id, provider, provider_user_id, email_at_link)
VALUES
  ('01935000-0000-7000-8000-000000900031', '01935000-0000-7000-8000-000000900011', 'google', 'seed-09-giau-bui', 'giau.bui@quizzivy.com'),
  ('01935000-0000-7000-8000-000000900032', '01935000-0000-7000-8000-000000900012', 'google', 'seed-09-duc-phan', 'duc.phan@quizzivy.com'),
  ('01935000-0000-7000-8000-000000900033', '01935000-0000-7000-8000-000000900025', 'google', 'seed-09-chi-mai', 'chi.mai@quizzivy.com')
ON CONFLICT DO NOTHING;

-- ------------------------------------------------------------------ classes

INSERT INTO app.classes (id, name, description, self_join_enabled, teacher_id, schedule_label, room, archived_at, created_at)
VALUES
  ('01935000-0000-7000-8000-000000900101', 'IELTS Writing Clinic', NULL, true,
   '01935000-0000-7000-8000-000000900012', 'Thu, 19:30', 'P.204', NULL, now() - interval '40 days'),
  ('01935000-0000-7000-8000-000000900102', 'Business English B1', NULL, true,
   '01935000-0000-7000-8000-000000900012', 'Tue, 12:00', 'P.105', NULL, now() - interval '35 days'),
  ('01935000-0000-7000-8000-000000900103', 'Summer Camp 2026', NULL, false,
   '01935000-0000-7000-8000-000000900013', 'Jul – Aug', NULL, now() - interval '30 days', now() - interval '150 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
SELECT c.id::uuid, u.id, 'admin', c.teacher::uuid
  FROM (VALUES
    ('01935000-0000-7000-8000-000000900101', '01935000-0000-7000-8000-000000900012',
     ARRAY['an.lam@quizzivy.com', 'chi.mai@quizzivy.com', 'khoa.dinh@quizzivy.com', 'son.vu@quizzivy.com', 'dung.hoang@example.com']),
    ('01935000-0000-7000-8000-000000900102', '01935000-0000-7000-8000-000000900012',
     ARRAY['nhi.ta@quizzivy.com', 'phuc.luu@quizzivy.com', 'trang.le@example.com']),
    ('01935000-0000-7000-8000-000000900103', '01935000-0000-7000-8000-000000900013',
     ARRAY['bich.hoang@quizzivy.com', 'tam.truong@quizzivy.com', 'han.pham@example.com', 'an.lam@quizzivy.com'])
  ) AS c(id, teacher, emails)
  JOIN app.users u ON u.email = ANY (c.emails)
ON CONFLICT DO NOTHING;

-- --------------------------------------------------------------- custom role

INSERT INTO app.roles (id, name, description, icon, color, copied_from, created_by)
VALUES ('01935000-0000-7000-8000-000000900201', 'Tổ trưởng',
        'Giáo viên dẫn dắt một tổ: dạy như giáo viên và xem nhật ký hoạt động.',
        'crown', 'amber', (SELECT id FROM app.roles WHERE builtin_key = 'teacher'),
        '01935000-0000-7000-8000-0000000000a1')
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.role_permissions (role_id, permission_key)
SELECT '01935000-0000-7000-8000-000000900201'::uuid, rp.permission_key
  FROM app.role_permissions rp
  JOIN app.roles r ON r.id = rp.role_id AND r.builtin_key = 'teacher'
UNION
SELECT '01935000-0000-7000-8000-000000900201'::uuid, p.key
  FROM app.permissions p WHERE p.key = 'system.audit.read' AND p.in_matrix
ON CONFLICT DO NOTHING;

-- -------------------------------------------------------------- audit log

INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
SELECT e.actor::uuid, e.action, e.entity, e.entity_id::uuid, now() - e.ago::interval, e.ip::inet,
       'seed/09-dev-admin-console', e.diff::jsonb
  FROM (VALUES
    ('01935000-0000-7000-8000-000000000601', 'attempt.extended', 'attempt', '01935000-0000-7000-8000-00000000dd07',
     '35 minutes', '14.161.22.8', '{"ends_at": {"old": "21:00", "new": "21:10"}}'),
    ('01935000-0000-7000-8000-0000000000a1', 'user.password_reset', 'user', '01935000-0000-7000-8000-000000800107',
     '2 hours', '113.22.4.19', '{"must_change_password": {"old": false, "new": true}}'),
    ('01935000-0000-7000-8000-000000000601', 'test.published', 'test', '01935000-0000-7000-8000-000000800501',
     '5 hours', '14.161.22.8', '{"status": {"old": "draft", "new": "published"}, "version": {"old": 2, "new": 3}}'),
    ('01935000-0000-7000-8000-000000000601', 'class.join_code_rotated', 'class', '01935000-0000-7000-8000-000000800301',
     '9 hours', '14.161.22.8', '{"code": {"old": "H2LV", "new": "K7QM"}}'),
    ('01935000-0000-7000-8000-000000900012', 'attempt.voided', 'attempt', '01935000-0000-7000-8000-00000000ee05',
     '1 day', '27.72.100.3', '{"status": {"old": "submitted", "new": "voided"}}'),
    ('01935000-0000-7000-8000-0000000000a1', 'user.disabled', 'user', '01935000-0000-7000-8000-000000900013',
     '27 days', '113.22.4.19', '{"disabled_at": {"old": null, "new": "disabled"}}'),
    ('01935000-0000-7000-8000-000000000601', 'class.member_added', 'class_member', '01935000-0000-7000-8000-000000800302',
     '1 day 6 hours', '14.161.22.8', '{"members": {"old": 13, "new": 16}}'),
    ('01935000-0000-7000-8000-000000000601', 'media.deleted', 'media_asset', '01935000-0000-7000-8000-000000800402',
     '2 days', '14.161.22.8', '{"used_in": {"old": "0 questions", "new": "deleted"}}'),
    (NULL, 'import.files_removed', 'word_import', '01935000-0000-7000-8000-000000800801',
     '2 days 18 hours', NULL, '{"files": {"old": "2 files", "new": "removed"}}'),
    ('01935000-0000-7000-8000-0000000000a1', 'user.role_changed', 'user', '01935000-0000-7000-8000-000000900012',
     '3 days', '113.22.4.19', '{"role": {"old": "student", "new": "teacher"}}'),
    ('01935000-0000-7000-8000-0000000000a1', 'role.created', 'role', '01935000-0000-7000-8000-000000900201',
     '4 days', '113.22.4.19', NULL),
    (NULL, 'retention.answers_removed', 'attempt_answers', NULL,
     '5 days', NULL, '{"answers": 120}')
  ) AS e(actor, action, entity, entity_id, ago, ip, diff)
 WHERE NOT EXISTS (SELECT 1 FROM app.audit_log WHERE user_agent = 'seed/09-dev-admin-console');

-- ------------------------------------------------------------------ activity

INSERT INTO app.user_activity_days (user_id, day)
SELECT u.id, current_date - d
  FROM generate_series(0, 13) AS d
  JOIN app.users u ON u.disabled_at IS NULL AND u.last_sign_in_at IS NOT NULL
 WHERE abs(hashtext(u.id::text || d::text)) % 10 < CASE WHEN extract(isodow FROM current_date - d) > 5 THEN 4 ELSE 7 END
ON CONFLICT DO NOTHING;

-- ---------------------------------------------------------------- AI credits

INSERT INTO app.ai_credit_periods (month, pool_credits, used_credits)
VALUES (date_trunc('month', now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date,
        (SELECT pool_credits FROM app.ai_settings), 0)
ON CONFLICT (month) DO NOTHING;

INSERT INTO app.ai_teacher_credits (month, user_id, allowance_credits, custom, paused, used_credits)
SELECT date_trunc('month', now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date, v.id::uuid, v.allowance, v.custom, v.paused, v.used
  FROM (VALUES
    ('01935000-0000-7000-8000-000000000601', 20000, false, false,  6240),
    ('01935000-0000-7000-8000-000000900011', 20000, false, false, 18420),
    ('01935000-0000-7000-8000-000000900012', 20000, false, false, 20000),
    ('01935000-0000-7000-8000-000000900014', 80000, true,  false, 30100),
    ('01935000-0000-7000-8000-000000900015', 20000, false, false, 12880),
    ('01935000-0000-7000-8000-000000900016', 20000, true,  true,  20000),
    ('01935000-0000-7000-8000-000000900017', 20000, false, false,  2140)
  ) AS v(id, allowance, custom, paused, used)
ON CONFLICT DO NOTHING;

UPDATE app.ai_credit_periods p
   SET used_credits = (SELECT coalesce(sum(t.used_credits), 0) FROM app.ai_teacher_credits t WHERE t.month = p.month)
 WHERE p.month = date_trunc('month', now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date
   AND p.used_credits = 0;

INSERT INTO app.ai_credit_requests (id, user_id, month, amount_credits, reason, created_at)
VALUES ('01935000-0000-7000-8000-000000900301', '01935000-0000-7000-8000-000000900012',
        date_trunc('month', now() AT TIME ZONE 'Asia/Ho_Chi_Minh')::date, 20000,
        'Chấm bài Writing Task 2 thi thử cho hai lớp trong tuần này.', now() - interval '2 hours')
ON CONFLICT (id) DO NOTHING;
