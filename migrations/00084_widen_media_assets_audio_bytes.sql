-- +goose Up

-- Audio may be 50 MiB; an image stays at 10 MiB (DG-63, T-R4.17a). 00010
-- declared the old rule as an unnamed column CHECK, which PostgreSQL named
-- media_assets_bytes_check. The new rule is added first, so no moment has
-- neither.
ALTER TABLE app.media_assets
  ADD CONSTRAINT media_assets_bytes_by_kind
    CHECK (bytes > 0 AND bytes <= CASE WHEN kind = 'audio' THEN 52428800 ELSE 10485760 END)
    NOT VALID;
ALTER TABLE app.media_assets VALIDATE CONSTRAINT media_assets_bytes_by_kind;
ALTER TABLE app.media_assets DROP CONSTRAINT media_assets_bytes_check;

-- +goose Down

-- NOT VALID: audio above 10 MiB may exist by now, and a validated constraint
-- would refuse to be added. It still binds every row written afterwards.
ALTER TABLE app.media_assets
  ADD CONSTRAINT media_assets_bytes_check
    CHECK (bytes > 0 AND bytes <= 10485760) NOT VALID;
ALTER TABLE app.media_assets DROP CONSTRAINT media_assets_bytes_by_kind;
