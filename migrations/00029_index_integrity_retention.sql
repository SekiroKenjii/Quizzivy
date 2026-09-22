-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY attempt_events_retention_idx ON app.attempt_events (received_at, id);

-- +goose Down
DROP INDEX CONCURRENTLY app.attempt_events_retention_idx;
