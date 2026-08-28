-- Development seed. Never run against production.
--
-- Seeds live here and never in a migration (§13.7): migrations describe the
-- schema every environment shares, seeds describe data only one of them wants.
-- `make seed` applies every .sql in this directory in name order.
--
-- Idempotent, so re-running after new migrations does not fail.

-- ---------------------------------------------------------------- admin
--
-- Password: quizzivy-dev
--
-- The hash is Argon2id in PHC string format, which is self-describing: the
-- parameters (m=65536,t=3,p=2) travel with the hash, so whatever defaults
-- T-1.2 chooses at runtime, this still verifies. A bare hash would silently
-- stop matching the moment those parameters were tuned.
INSERT INTO app.users (id, email, full_name, role, password_hash, must_change_password)
VALUES (
  '01935000-0000-7000-8000-0000000000a1',
  'thuong@quizzivy.com',
  'Thuong',
  'admin',
  '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
  false
)
ON CONFLICT (id) DO NOTHING;

-- ---------------------------------------------------------------- student
--
-- Same password. Exists so the §5.4 guards have something to redirect: an
-- `admin` on /app/* and a `student` on /admin/* behave differently, and one
-- account cannot exercise both.
INSERT INTO app.users (id, email, full_name, role, password_hash, must_change_password)
VALUES (
  '01935000-0000-7000-8000-0000000000a2',
  'hocvien@quizzivy.com',
  'Nguyễn Văn An',
  'student',
  '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
  false
)
ON CONFLICT (id) DO NOTHING;

-- ------------------------------------------------------------------ class
INSERT INTO app.classes (id, name, description, self_join_enabled)
VALUES (
  '01935000-0000-7000-8000-0000000000c1',
  'Tiếng Anh giao tiếp — Lớp A',
  'Lớp mẫu để phát triển.',
  true
)
ON CONFLICT (id) DO NOTHING;

-- The student is enrolled by the admin, so joined_via is 'admin' and
-- join_code_id stays NULL -- the pairing class_members_source_consistent
-- requires. A code-based enrolment is seeded by T-1.6, once codes can be
-- generated and hashed.
INSERT INTO app.class_members (class_id, user_id, joined_via, added_by)
VALUES (
  '01935000-0000-7000-8000-0000000000c1',
  '01935000-0000-7000-8000-0000000000a2',
  'admin',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (class_id, user_id) DO NOTHING;
