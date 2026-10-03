-- +goose Up

-- Who created an account (T-R2.9). NULL means the provenance is unknown: every
-- account that exists today, and a Google self-join, which has no creator.
-- There is nothing to backfill from, and deleting the creator must not take
-- the account with it.
ALTER TABLE app.users ADD COLUMN created_by uuid REFERENCES app.users ON DELETE SET NULL;

-- +goose Down

ALTER TABLE app.users DROP COLUMN created_by;
