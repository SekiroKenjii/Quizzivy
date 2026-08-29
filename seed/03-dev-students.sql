-- Development seed, part three: enough students for G-07 to be worth looking at.
--
-- Three profiles, chosen so every rendering of every column appears at least
-- once: a live attempt, a graded score, and a student who has never started
-- anything. Without the third, the em dash path is never exercised by hand.
--
-- Self-contained and idempotent, like the other two. Fixed uuids throughout.

-- --------------------------------------------------------------- students
INSERT INTO app.users (id, email, full_name, role, password_hash, must_change_password)
VALUES
  -- Google-only: no password_hash, which is what makes the drawer offer
  -- "Đặt mật khẩu tạm" with the Google-only wording.
  ('01935000-0000-7000-8000-00000000ee01'::uuid,
   'han.pham@example.com', 'Phạm Gia Hân', 'student', NULL, false),
  ('01935000-0000-7000-8000-00000000ee02'::uuid,
   'dung.hoang@example.com', 'Hoàng Tiến Dũng', 'student',
   '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
   false),
  ('01935000-0000-7000-8000-00000000ee03'::uuid,
   'trang.le@example.com', 'Lê Thu Trang', 'student',
   '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
   false)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.user_identities (user_id, provider, provider_user_id, email_at_link)
VALUES ('01935000-0000-7000-8000-00000000ee01'::uuid, 'google',
        'dev-google-sub-han', 'han.pham@example.com')
ON CONFLICT (user_id, provider) DO NOTHING;

INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
VALUES
  ('01935000-0000-7000-8000-0000000000c1'::uuid,
   '01935000-0000-7000-8000-00000000ee01'::uuid, 'admin',
   '01935000-0000-7000-8000-0000000000a1'::uuid),
  ('01935000-0000-7000-8000-0000000000c1'::uuid,
   '01935000-0000-7000-8000-00000000ee02'::uuid, 'admin',
   '01935000-0000-7000-8000-0000000000a1'::uuid)
ON CONFLICT (class_id, user_id) DO NOTHING;
-- Lê Thu Trang stays out of every class on purpose: the "—" in the Lớp column
-- is a state the deck never draws and therefore the one most likely to break.

-- ------------------------------------------------- a second, closed window
-- Closed and fully graded, so there is something for "Điểm TB" to average that
-- is not the open assignment 02 already seeds.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000ee04'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  now() - interval '9 days',
  now() - interval '7 days',
  45,
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
VALUES ('01935000-0000-7000-8000-00000000ee04'::uuid,
        '01935000-0000-7000-8000-0000000000c1'::uuid)
ON CONFLICT DO NOTHING;

-- -------------------------------------------------------------- attempts
-- Dũng: graded twice on the same assignment. The lower attempt is deliberately
-- the LATER one, so a projection that takes the latest rather than the best
-- shows 40% instead of 60% and the seed catches it by eye.
INSERT INTO app.attempts
  (id, assignment_id, test_version_id, student_id, attempt_no, status,
   session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at,
   submitted_at, graded_at, score_earned, score_total, flagged)
VALUES
  ('01935000-0000-7000-8000-00000000ee05'::uuid,
   '01935000-0000-7000-8000-00000000ee04'::uuid,
   '01935000-0000-7000-8000-00000000dd02'::uuid,
   '01935000-0000-7000-8000-00000000ee02'::uuid,
   1, 'graded', gen_random_uuid(), 11, sha256('dev-ee05'::bytea),
   now() - interval '9 days', now() - interval '9 days' + interval '45 minutes',
   now() - interval '9 days' + interval '30 minutes',
   now() - interval '8 days', '2.40', '4.00', false),
  ('01935000-0000-7000-8000-00000000ee06'::uuid,
   '01935000-0000-7000-8000-00000000ee04'::uuid,
   '01935000-0000-7000-8000-00000000dd02'::uuid,
   '01935000-0000-7000-8000-00000000ee02'::uuid,
   2, 'graded', gen_random_uuid(), 12, sha256('dev-ee06'::bytea),
   now() - interval '8 days', now() - interval '8 days' + interval '45 minutes',
   now() - interval '8 days' + interval '20 minutes',
   now() - interval '8 days', '1.60', '4.00', false)
ON CONFLICT (id) DO NOTHING;

-- Hân: mid-attempt right now on the open assignment, and flagged. Renders as
-- "đang làm bài" with no score, which is the row the deck draws.
INSERT INTO app.attempts
  (id, assignment_id, test_version_id, student_id, attempt_no, status,
   session_id, shuffle_seed, beacon_token_hash, started_at, deadline_at,
   flagged, focus_loss_count)
VALUES (
  '01935000-0000-7000-8000-00000000ee07'::uuid,
  '01935000-0000-7000-8000-00000000dd06'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  '01935000-0000-7000-8000-00000000ee01'::uuid,
  1, 'in_progress', gen_random_uuid(), 13, sha256('dev-ee07'::bytea),
  now() - interval '12 minutes', now() + interval '33 minutes',
  true, 3
)
ON CONFLICT (id) DO NOTHING;
