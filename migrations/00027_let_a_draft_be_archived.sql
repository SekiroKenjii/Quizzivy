-- +goose Up

-- 00015 tied "has a snapshot" to every non-draft status, which also refused
-- to archive a draft that was never published -- the exact test a teacher
-- tidies away. Only a PUBLISHED test can be pointed at by an assignment, so
-- only that status needs a version behind it (#72).
ALTER TABLE app.tests DROP CONSTRAINT tests_published_has_version;
ALTER TABLE app.tests ADD CONSTRAINT tests_published_has_version
  CHECK (status <> 'published' OR current_version > 0);

-- +goose Down

-- A never-published archive cannot satisfy the older rule; it goes back to
-- being a draft rather than blocking the rollback.
UPDATE app.tests SET status = 'draft' WHERE status = 'archived' AND current_version = 0;
ALTER TABLE app.tests DROP CONSTRAINT tests_published_has_version;
ALTER TABLE app.tests ADD CONSTRAINT tests_published_has_version
  CHECK (status = 'draft' OR current_version > 0);
