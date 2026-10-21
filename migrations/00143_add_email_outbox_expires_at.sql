-- +goose Up

-- When a message's link stops working (D-060). A monthly email cap can hold a
-- message until the 1st of the next month, longer than a set-password or reset
-- link or an export's download lives, so the dispatcher fails a row whose link
-- has expired, with code 'expired' and its sealed secret erased, rather than
-- send it. NULL is a message with no link that expires.
ALTER TABLE app.email_outbox ADD COLUMN expires_at timestamptz;

-- Which cap holds a message back (D-061): set when a spent daily or monthly
-- budget moves a due message to the next day or month, cleared when budget
-- remains and dispatch releases it, so a raised cap sends it at once. A held
-- message whose link expires keeps the mark as the record of why it was never
-- sent; a sent message never has one.
ALTER TABLE app.email_outbox
  ADD COLUMN held_by text,
  ADD CONSTRAINT email_outbox_held_by_check CHECK (held_by IN ('day', 'month')),
  ADD CONSTRAINT email_outbox_held_unsent CHECK (held_by IS NULL OR sent_at IS NULL);

-- +goose Down
ALTER TABLE app.email_outbox DROP COLUMN held_by;
ALTER TABLE app.email_outbox DROP COLUMN expires_at;
