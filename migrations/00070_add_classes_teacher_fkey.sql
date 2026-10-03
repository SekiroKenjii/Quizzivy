-- +goose Up

-- The foreign key for classes.teacher_id (T-R2.9), in its own transaction
-- rather than inline in 00065. 00065 holds ACCESS EXCLUSIVE on app.classes
-- until it commits, and an inline REFERENCES would then wait for SHARE ROW
-- EXCLUSIVE on app.users. Meanwhile v0.7.0's student creation writes app.users
-- and then checks each chosen class through class_members' foreign key, which
-- needs a lock on app.classes: each would wait on the other, and the deadlock
-- would abort the release. Here the migration takes SHARE ROW EXCLUSIVE on
-- app.classes, which reads and foreign-key checks do not conflict with, so that
-- transaction finishes first and this one waits for it.
ALTER TABLE app.classes ADD CONSTRAINT classes_teacher_id_fkey
  FOREIGN KEY (teacher_id) REFERENCES app.users ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE app.classes DROP CONSTRAINT classes_teacher_id_fkey;
