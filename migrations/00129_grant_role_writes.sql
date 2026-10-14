-- +goose Up

-- The role commands' writes (T-R5.8, plan 75 section "Roles and permissions
-- API"). 00054 left the app role read-only on the three access tables; the
-- Admin console now creates and edits roles and turns grants on and off.
--
-- UPDATE on roles is limited to the columns a role command changes. DELETE on
-- roles is never granted (DG-52), so no role disappears, and app.permissions
-- stays read-only. role_permissions_bump_revision runs as the invoker and
-- updates roles.revision, which is why that column is in the list.
-- users_last_admin does not need a grant of its own: its function is SECURITY
-- DEFINER (20-data-model.md section 30).
GRANT INSERT, UPDATE (name, description, icon, color, revision, updated_at) ON app.roles TO quizzivy_app;
GRANT INSERT, DELETE ON app.role_permissions TO quizzivy_app;

-- +goose Down

REVOKE INSERT, UPDATE (name, description, icon, color, revision, updated_at) ON app.roles FROM quizzivy_app;
REVOKE INSERT, DELETE ON app.role_permissions FROM quizzivy_app;
