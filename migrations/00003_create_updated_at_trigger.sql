-- +goose Up

-- §13.2: `updated_at` via trigger, never application code.
--
-- One function, reused by every table that has the column. Application-managed
-- timestamps drift the moment one code path forgets, and the tables where it
-- matters most -- attempt_answers during autosave -- are the ones with the most
-- write paths.
-- +goose StatementBegin
CREATE FUNCTION app.set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $fn$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$fn$;
-- +goose StatementEnd

-- +goose Down

DROP FUNCTION IF EXISTS app.set_updated_at();
