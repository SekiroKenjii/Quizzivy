-- +goose Up

-- T-R5.4 (DG-54): when an account was last active, last signed in and how,
-- and when its password last changed. All four are nullable, so a v0.10.0
-- binary's INSERT, which names none of them, still succeeds while this
-- release rolls out. last_sign_in_at IS NULL is "Not signed in".
ALTER TABLE app.users
  ADD COLUMN last_active_at timestamptz,
  ADD COLUMN last_sign_in_at timestamptz,
  ADD COLUMN last_sign_in_method text,
  ADD COLUMN password_changed_at timestamptz,
  ADD CONSTRAINT users_last_sign_in_method_check
    CHECK (last_sign_in_method IN ('password', 'google', 'set_password_link'));

-- An account used before R5 must not read "Never signed in": both stamps
-- start at the newest of its newest refresh token, its newest attempt and its
-- newest audit entry as actor. The method stays unknown. refresh_tokens has
-- no created_at; issued_at is the time a token was minted. Each source is
-- read through its (user, time) index, and the trigger is off so that
-- updated_at keeps meaning "the account was edited".
ALTER TABLE app.users DISABLE TRIGGER users_set_updated_at;

UPDATE app.users u
   SET last_sign_in_at = seen.at,
       last_active_at = seen.at
  FROM (
    SELECT x.id, greatest(
             (SELECT max(t.issued_at) FROM app.refresh_tokens t WHERE t.user_id = x.id),
             (SELECT max(a.started_at) FROM app.attempts a WHERE a.student_id = x.id),
             (SELECT max(l.occurred_at) FROM app.audit_log l WHERE l.actor_user_id = x.id)
           ) AS at
      FROM app.users x
  ) AS seen
 WHERE seen.id = u.id
   AND seen.at IS NOT NULL;

ALTER TABLE app.users ENABLE TRIGGER users_set_updated_at;

-- +goose Down

ALTER TABLE app.users
  DROP COLUMN last_active_at,
  DROP COLUMN last_sign_in_at,
  DROP COLUMN last_sign_in_method,
  DROP COLUMN password_changed_at;
