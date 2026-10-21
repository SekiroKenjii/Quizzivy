-- +goose Up

-- When a message's link stops working (D-060). A monthly email cap can hold a
-- message until the 1st of the next month, longer than a set-password or reset
-- link or an export's download lives, so the dispatcher fails a row whose link
-- has expired, with code 'expired' and its sealed secret erased, rather than
-- send it. NULL is a message with no link that expires.
ALTER TABLE app.email_outbox ADD COLUMN expires_at timestamptz;

-- +goose Down
ALTER TABLE app.email_outbox DROP COLUMN expires_at;
