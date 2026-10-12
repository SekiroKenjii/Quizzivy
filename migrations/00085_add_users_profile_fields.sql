-- +goose Up
ALTER TABLE app.users
    ADD COLUMN display_name text,
    ADD COLUMN phone text,
    ADD COLUMN avatar_key text,
    ADD COLUMN locale text,
    ADD COLUMN time_zone text,
    ADD CONSTRAINT users_display_name_check CHECK (char_length(btrim(display_name)) BETWEEN 1 AND 80),
    ADD CONSTRAINT users_phone_check CHECK (phone ~ '^[0-9+ ]{6,20}$'),
    ADD CONSTRAINT users_locale_check CHECK (locale IN ('vi', 'en')),
    ADD CONSTRAINT users_time_zone_check CHECK (char_length(time_zone) BETWEEN 1 AND 64);

-- +goose Down
ALTER TABLE app.users
    DROP COLUMN display_name,
    DROP COLUMN phone,
    DROP COLUMN avatar_key,
    DROP COLUMN locale,
    DROP COLUMN time_zone;
