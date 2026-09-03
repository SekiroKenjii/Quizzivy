-- Fixtures for the live E2E suite (E2E 5, 6, 7).
--
-- Separate from 02-dev-assignments.sql because these exist to be consumed:
-- every run starts an attempt against them, and their policies are set to the
-- extremes the scenarios need rather than to anything a teacher would choose.
-- E2E 8 is not here — it needs a real audio object, so it authors its own test
-- through the admin UI.
--
-- They share 02's published test version, so the frozen-question invariants in
-- 99-assert.sql already hold for them.
--
-- max_attempts is 50 rather than 1: a developer runs the suite repeatedly
-- against one database, and an exhausted assignment fails as "hết lượt" rather
-- than as the thing under test.

-- --------------------------------------------------------- E2E 5 · the timer
-- One minute is the contract's floor (duration_minutes BETWEEN 1 AND 600), and
-- the deadline is min(startedAt + duration, closesAt), so this is the shortest
-- attempt the product can express.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes,
   max_attempts, published_at, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000ee01'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  now() - interval '1 hour',
  now() + interval '30 days',
  1,
  50,
  now() - interval '1 hour',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

-- ----------------------------------------------------- E2E 6 · the away rule
-- min_away_ms 0 counts any blur/focus pair, and max_focus_loss 1 puts the
-- student one episode from the limit, so the dialog appears on the first one.
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes,
   max_attempts, integrity_max_focus_loss, integrity_min_away_ms,
   integrity_on_limit_exceeded, published_at, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000ee02'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  now() - interval '1 hour',
  now() + interval '30 days',
  45,
  50,
  1,
  0,
  'flag',
  now() - interval '1 hour',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

-- ------------------------------------------------- E2E 7 · session takeover
INSERT INTO app.assignments
  (id, test_id, test_version_id, opens_at, closes_at, duration_minutes,
   max_attempts, published_at, created_by)
VALUES (
  '01935000-0000-7000-8000-00000000ee03'::uuid,
  '01935000-0000-7000-8000-00000000dd01'::uuid,
  '01935000-0000-7000-8000-00000000dd02'::uuid,
  now() - interval '1 hour',
  now() + interval '30 days',
  45,
  50,
  now() - interval '1 hour',
  '01935000-0000-7000-8000-0000000000a1'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO app.assignment_classes (assignment_id, class_id)
SELECT a, '01935000-0000-7000-8000-0000000000c1'::uuid
  FROM unnest(ARRAY[
    '01935000-0000-7000-8000-00000000ee01'::uuid,
    '01935000-0000-7000-8000-00000000ee02'::uuid,
    '01935000-0000-7000-8000-00000000ee03'::uuid
  ]) AS a
ON CONFLICT DO NOTHING;
