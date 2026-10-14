-- +goose NO TRANSACTION
-- +goose Up

-- The order the Admin console reads the audit log in: newest first, ties broken
-- by id, over a range of time. audit_log_entity_idx and audit_log_actor_idx
-- lead with the entity and the actor, so neither can give this order; without
-- this index every page of GET /admin/audit-log sorts the whole range, and the
-- export's keyset batches (occurred_at, id) < ($1, $2) walk it. Both columns
-- descend so a backward scan is not needed to read them newest first. CONCURRENTLY,
-- because the log is populated and written to all the time, and one index to the
-- file, so an interrupted build leaves at most this one behind (docs/setup/deploy.md).
-- Grants are untouched: the app role still has INSERT and SELECT on the log and
-- nothing else.
CREATE INDEX CONCURRENTLY IF NOT EXISTS audit_log_recent_idx
  ON app.audit_log (occurred_at DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.audit_log_recent_idx;
