-- +goose Up

-- What the Media library shows and keeps for a file beside its bytes
-- (T-R4.17a). All three are nullable with no default, so the previous
-- release's insert, which names none of them, keeps working during the
-- rolling deploy.
--
-- display_name: the name the library shows; NULL means the original filename.
-- default_max_plays: the play limit a new question starts from. NULL is not
--   set, 0 is unlimited; audio only.
-- replaced_by: the file that took this one's place (T-R4.17b writes it). A
--   replaced file leaves the library and keeps serving what still points at
--   it. SET NULL, so removing the newer row returns the older to the library.
--   No index: the table is small, and a row is never hard-deleted outside
--   tests.
ALTER TABLE app.media_assets
  ADD COLUMN display_name text,
  ADD COLUMN default_max_plays smallint,
  ADD COLUMN replaced_by uuid,
  ADD CONSTRAINT media_assets_display_name_check
    CHECK (char_length(display_name) BETWEEN 1 AND 200 AND display_name ~ '\S'),
  ADD CONSTRAINT media_assets_default_max_plays_check
    CHECK (default_max_plays IS NULL
           OR (default_max_plays BETWEEN 0 AND 3 AND kind = 'audio')),
  ADD CONSTRAINT media_assets_replaced_by_fkey
    FOREIGN KEY (replaced_by) REFERENCES app.media_assets(id) ON DELETE SET NULL,
  ADD CONSTRAINT media_assets_replaced_by_check
    CHECK (replaced_by <> id);

-- +goose Down

ALTER TABLE app.media_assets
  DROP COLUMN replaced_by,
  DROP COLUMN default_max_plays,
  DROP COLUMN display_name;
