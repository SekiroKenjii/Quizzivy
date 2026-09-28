-- +goose NO TRANSACTION
-- +goose Up

-- A teacher's media library by kind, newest first, as 00010's
-- media_assets_kind_created_idx serves for everyone (T-R2.12c). CONCURRENTLY,
-- because the table is populated and in use; one index per file, so an
-- interrupted build leaves at most this one behind (T-R2.10).
CREATE INDEX CONCURRENTLY media_assets_owner_idx
  ON app.media_assets (owner_id, kind, created_at DESC)
  WHERE deleted_at IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.media_assets_owner_idx;
