-- Development seed, part seven: a whole classroom (R2, T-R2.11).
--
-- Relies on nothing earlier. Thirty students, hocvien.lop01@quizzivy.com to
-- hocvien.lop30@quizzivy.com, password quizzivy-dev, no forced change, and in
-- no class: T-R2.15's live test joins all thirty to one class by code from a
-- single address, as a school's shared connection would.
--
-- Fixed uuids (…000000000701 to …000000000730). ON CONFLICT DO NOTHING has no
-- target, so an account that already holds one of these emails is left alone.

INSERT INTO app.users (id, email, full_name, role_id, password_hash, must_change_password)
SELECT ('01935000-0000-7000-8000-0000000007' || lpad(n::text, 2, '0'))::uuid,
       'hocvien.lop' || lpad(n::text, 2, '0') || '@quizzivy.com',
       'Học viên ' || lpad(n::text, 2, '0'),
       (SELECT id FROM app.roles WHERE builtin_key = 'student'),
       '$argon2id$v=19$m=65536,t=3,p=2$NsEIYu5N8g+iv1W9zV2hfQ$HgTGHdo9uosWEPKpMFDPDSUvBOTCc0oVcPvq7FeVIR4',
       false
  FROM generate_series(1, 30) AS n
ON CONFLICT DO NOTHING;
