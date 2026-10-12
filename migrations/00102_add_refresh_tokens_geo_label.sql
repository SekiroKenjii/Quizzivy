-- +goose Up

-- Where a signed-in device last signed in or refreshed, as "City, CC" (T-R4.9),
-- for the Signed-in devices list. Nullable with no default, so the previous
-- release's insert keeps working; a row made before this column, or without
-- trusted location headers, reads NULL. The command stores NULL, never an
-- empty string, so the check meets only 1 to 80 characters. The check scans
-- the table once while the column is added; refresh_tokens is pruned by
-- expiry, so the scan is cheap.
ALTER TABLE app.refresh_tokens
  ADD COLUMN geo_label text
    CONSTRAINT refresh_tokens_geo_label_check
    CHECK (char_length(geo_label) BETWEEN 1 AND 80);

-- +goose Down

ALTER TABLE app.refresh_tokens
  DROP CONSTRAINT refresh_tokens_geo_label_check,
  DROP COLUMN geo_label;
