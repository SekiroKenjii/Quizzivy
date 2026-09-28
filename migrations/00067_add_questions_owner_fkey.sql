-- +goose Up

-- The foreign key for questions.owner_id (T-R2.9), in its own transaction
-- rather than inline in 00061. 00061 holds ACCESS EXCLUSIVE on app.questions
-- until it commits, and an inline REFERENCES would then wait for SHARE ROW
-- EXCLUSIVE on app.users. Meanwhile v0.7.0 deleting a user holds app.users and
-- then checks the rows in app.questions that reference it, which needs a lock
-- on app.questions: each would wait on the other, and the deadlock would abort
-- the release. Here the migration takes SHARE ROW EXCLUSIVE on app.questions,
-- which reads and foreign-key checks do not conflict with, so that transaction
-- finishes first and this one waits for it.
ALTER TABLE app.questions ADD CONSTRAINT questions_owner_id_fkey
  FOREIGN KEY (owner_id) REFERENCES app.users ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE app.questions DROP CONSTRAINT questions_owner_id_fkey;
