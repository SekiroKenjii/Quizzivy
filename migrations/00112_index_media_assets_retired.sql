-- +goose NO TRANSACTION
-- +goose Up

-- The files that have left the library and are still in storage: the usage
-- figures read one owner's, and the sweep walks them all. A file leaves this
-- index when it is purged, so the index holds the few rows a library is
-- waiting to be rid of and not the library. CONCURRENTLY, because the table is
-- populated and in use, and one index to the file, so an interrupted build
-- leaves at most this one behind (F-68).
CREATE INDEX CONCURRENTLY media_assets_retired_idx
  ON app.media_assets (owner_id)
  WHERE purged_at IS NULL AND (deleted_at IS NOT NULL OR replaced_by IS NOT NULL);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.media_assets_retired_idx;
