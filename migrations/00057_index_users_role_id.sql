-- +goose NO TRANSACTION
-- +goose Up

-- CONCURRENTLY, because app.users is populated and in use (plan 70 §3). The
-- partial predicate matches the active-member counts that replace
-- users_role_active_idx when R3 drops users.role.
CREATE INDEX CONCURRENTLY users_role_id_active_idx ON app.users (role_id) WHERE disabled_at IS NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS app.users_role_id_active_idx;
