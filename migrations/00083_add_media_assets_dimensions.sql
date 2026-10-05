-- +goose Up

-- An image's size in pixels, read from its header at upload (T-R4.17a). Both
-- are NULL for audio, for an image whose header could not be read, and for
-- every row stored before this migration.
ALTER TABLE app.media_assets
  ADD COLUMN width integer,
  ADD COLUMN height integer,
  ADD CONSTRAINT media_assets_dimensions_paired
    CHECK ((width IS NULL) = (height IS NULL)),
  ADD CONSTRAINT media_assets_dimensions_positive
    CHECK (width > 0 AND height > 0),
  ADD CONSTRAINT media_assets_dimensions_image_only
    CHECK (width IS NULL OR kind = 'image');

-- +goose Down

ALTER TABLE app.media_assets
  DROP COLUMN height,
  DROP COLUMN width;
