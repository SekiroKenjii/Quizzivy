-- +goose NO TRANSACTION
-- +goose Up

-- The order the Admin console reads the audit log in: newest first, ties broken
-- by id, over a range of time. audit_log_entity_idx and audit_log_actor_idx
-- lead with the entity and the actor, so neither can give this order; without
-- this index every page of GET /admin/audit-log sorts the whole range, and the
-- export's keyset batches (occurred_at, id) < ($1, $2) walk it. Both columns
-- descend so a backward scan is not needed to read them newest first. CONCURRENTLY,
-- because the log is populated and written to all the time.
--
-- A build that is interrupted leaves the index INVALID, which the planner
-- ignores, and goose has not recorded this file. The drop comes first so the
-- rerun removes that leftover and builds the index again: with IF NOT EXISTS
-- alone the rerun finds the name taken, does nothing, and records the file as
-- applied over an index that is never used. Nothing here runs again once the
-- file is recorded, so a valid index is dropped only when a build crashed after
-- finishing and before goose wrote its row.
--
-- Grants are untouched: the app role still has INSERT and SELECT on the log and
-- nothing else.
DROP INDEX CONCURRENTLY IF EXISTS app.audit_log_recent_idx;

CREATE INDEX CONCURRENTLY audit_log_recent_idx
  ON app.audit_log (occurred_at DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.audit_log_recent_idx;
