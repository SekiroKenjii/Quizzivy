-- +goose Up

-- What a replaced or deleted file's life after the library needs (F-68).
--
-- unreferenced_at: when the file was last seen to be named by nothing. It is
--   written when the file leaves the library with nothing naming it, and by the
--   sweep when it first finds that true of a file something named until then.
--   NULL means something still names it, or the file has not left the library.
--   A binding of the file clears it (00113). The sweep removes the object seven
--   days after this time, and only after checking again under the row lock.
-- purged_at: when the sweep removed the file's object from storage. The row
--   stays: audit rows, the replaced_by chain and the owner's history name it.
--   A purged file is named by nothing and can be bound again by nothing.
--
-- Both columns are nullable with no default, so the previous release's inserts,
-- which name neither, keep working during the rolling deploy. A purged file was
-- unreferenced first, which the check says.
ALTER TABLE app.media_assets
  ADD COLUMN unreferenced_at timestamptz,
  ADD COLUMN purged_at timestamptz,
  ADD CONSTRAINT media_assets_purged_check
    CHECK (purged_at IS NULL OR unreferenced_at IS NOT NULL);

-- +goose Down

ALTER TABLE app.media_assets
  DROP COLUMN purged_at,
  DROP COLUMN unreferenced_at;
