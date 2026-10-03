-- +goose Up

-- The one strict-student predicate (T-R2.1, plan 70 §4.3): the built-in
-- Student, or a custom role that holds nothing but learning.take_tests. Every
-- repository that picks a student target and core/maintenance read it. Built-in
-- roles other than Student are excluded by key, because the Admin stores only
-- its "Take tests" cell and would otherwise match.
CREATE VIEW app.student_like_roles AS
SELECT r.id
  FROM app.roles r
 WHERE r.builtin_key = 'student'
    OR (r.builtin_key IS NULL
        AND NOT EXISTS (SELECT 1
                          FROM app.role_permissions rp
                         WHERE rp.role_id = r.id
                           AND rp.permission_key <> 'learning.take_tests'));

-- A single-table view is automatically updatable, and 00009's default
-- privileges would let the app role delete roles through it.
REVOKE INSERT, UPDATE, DELETE ON app.student_like_roles FROM quizzivy_app;

-- +goose Down

DROP VIEW app.student_like_roles;
