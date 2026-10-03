-- +goose Up

-- The number an access token must carry to be honoured (T-R2.2). A command
-- that ends a user's access bumps it; in R2 that is a disable and a staff
-- password reset. A constant default is metadata-only on PG18.
ALTER TABLE app.users ADD COLUMN session_epoch integer NOT NULL DEFAULT 0
  CONSTRAINT users_session_epoch_check CHECK (session_epoch >= 0);

-- +goose Down

ALTER TABLE app.users DROP COLUMN session_epoch;
