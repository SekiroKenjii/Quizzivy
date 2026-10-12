-- +goose Up
ALTER TABLE app.users
    ADD COLUMN preferences jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD CONSTRAINT users_preferences_object_check CHECK (jsonb_typeof(preferences) = 'object'),
    ADD CONSTRAINT users_preferences_bytes_check CHECK (octet_length(preferences::text) <= 8192);

-- +goose Down
ALTER TABLE app.users DROP COLUMN preferences;
