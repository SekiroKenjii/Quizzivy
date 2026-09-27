-- +goose Up

-- The foreign key for tests.owner_id (T-R2.9), in its own transaction rather
-- than inline in 00060. 00060 holds ACCESS EXCLUSIVE on app.tests until it
-- commits, and an inline REFERENCES would then wait for SHARE ROW EXCLUSIVE on
-- app.users. Meanwhile v0.7.0 deleting a user holds app.users and then checks
-- the rows in app.tests that reference it, which needs a lock on app.tests:
-- each would wait on the other, and the deadlock would abort the release. Here
-- the migration takes SHARE ROW EXCLUSIVE on app.tests, which reads and
-- foreign-key checks do not conflict with, so that transaction finishes first
-- and this one waits for it.
ALTER TABLE app.tests ADD CONSTRAINT tests_owner_id_fkey
  FOREIGN KEY (owner_id) REFERENCES app.users ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE app.tests DROP CONSTRAINT tests_owner_id_fkey;
