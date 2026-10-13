-- +goose Up

-- The two permissions of the AI assistant (T-D4.32, plan 70 section 4.1, D19 to
-- D22). ai.use is the Content row "Use AI features", held by the Admin
-- wildcard and granted to the built-in Teacher role; ai.credits.manage is the
-- System row "Manage AI credits", held by the Admin wildcard alone. Both are
-- matrix rows (DG-199), so a custom role may be granted either.
--
-- The system group already numbers its four hidden keys 4 to 7, and the
-- catalogue lists the hidden keys last. The new System row takes 4, so the
-- hidden keys move up by one. The unique (group_key, ordinal) is not
-- deferrable and a single UPDATE checks it row by row, so they move out of
-- the way first and back in as two statements.
UPDATE app.permissions SET ordinal = ordinal + 100 WHERE group_key = 'system' AND ordinal >= 4;
UPDATE app.permissions SET ordinal = ordinal - 99  WHERE group_key = 'system' AND ordinal >= 104;

INSERT INTO app.permissions (key, group_key, ordinal, in_matrix) VALUES
  ('ai.use',            'content', 6, true),
  ('ai.credits.manage', 'system',  4, true);

-- The Admin stores no grant (it is the wildcard), so only the Teacher is
-- written. The statement names the built-in role by its immutable key and
-- writes nothing on a database that lacks it.
INSERT INTO app.role_permissions (role_id, permission_key)
SELECT r.id, 'ai.use'
  FROM app.roles r
 WHERE r.builtin_key = 'teacher';

-- +goose Down

DELETE FROM app.role_permissions WHERE permission_key IN ('ai.use', 'ai.credits.manage');
DELETE FROM app.permissions WHERE key IN ('ai.use', 'ai.credits.manage');

UPDATE app.permissions SET ordinal = ordinal + 100 WHERE group_key = 'system' AND ordinal >= 5;
UPDATE app.permissions SET ordinal = ordinal - 101 WHERE group_key = 'system' AND ordinal >= 105;
