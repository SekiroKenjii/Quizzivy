-- +goose Up

-- Archive, not delete (G-08): memberships, attempts and grades hang off a
-- class, so the record stays and only the pickers stop offering it.
ALTER TABLE app.classes ADD COLUMN archived_at timestamptz;

-- +goose Down

ALTER TABLE app.classes DROP COLUMN archived_at;
