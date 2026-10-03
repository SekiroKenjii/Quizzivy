-- +goose Up

-- The foreign key for media_assets.owner_id (T-R2.9), in its own transaction
-- rather than inline in 00063. 00063 holds ACCESS EXCLUSIVE on app.media_assets
-- until it commits, and an inline REFERENCES would then wait for SHARE ROW
-- EXCLUSIVE on app.users. Meanwhile v0.7.0 deleting a user holds app.users and
-- then checks the rows in app.media_assets that reference it, which needs a
-- lock on app.media_assets: each would wait on the other, and the deadlock
-- would abort the release. Here the migration takes SHARE ROW EXCLUSIVE on
-- app.media_assets, which reads and foreign-key checks do not conflict with, so
-- that transaction finishes first and this one waits for it.
ALTER TABLE app.media_assets ADD CONSTRAINT media_assets_owner_id_fkey
  FOREIGN KEY (owner_id) REFERENCES app.users ON DELETE RESTRICT;

-- +goose Down

ALTER TABLE app.media_assets DROP CONSTRAINT media_assets_owner_id_fkey;
