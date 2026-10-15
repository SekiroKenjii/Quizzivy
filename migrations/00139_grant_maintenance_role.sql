-- +goose Up

-- What cmd/maintenance may do, granted to quizzivy_maintenance (T-R5.17a, D16).
-- scripts/provision-db.sh creates the role; this migration fails when it is
-- missing, and there is no conditional grant, so a database that migrates has
-- the role and a role without these grants cannot exist beside the schema.
--
-- Every grant is the least a command needs. A table is granted by the columns
-- its statements read or write: nothing here reads a password hash, a token, a
-- preference or a free-text column that no command compares, and a command
-- that changes one column holds UPDATE on that column. DELETE is granted only
-- where a command deletes, and the role may never UPDATE or DELETE audit_log,
-- which stays append-only for it as for quizzivy_app. Later releases that add
-- or widen a command grant what it needs in their own migration and pin it in
-- privileges_test.go. Beyond grants it adds one trigger on app.users, below,
-- that keeps this role from setting a credential.

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'quizzivy_maintenance') THEN
    RAISE EXCEPTION 'role quizzivy_maintenance does not exist: run scripts/provision-db.sh first'
      USING ERRCODE = 'undefined_object';
  END IF;
END
$$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA app TO quizzivy_maintenance;

-- app.immutable_unaccent calls public.unaccent, and the trigram indexes on the
-- name and email columns call it on every write, so a role that rewrites a
-- user needs to reach schema public. USAGE only: it may not create there.
GRANT USAGE ON SCHEMA public TO quizzivy_maintenance;

-- retention-sweep and retain-integrity: the settings, the attempts the
-- answers and focus-loss events are measured by, and the audit entry each
-- batch writes.
GRANT SELECT (keep_answers_months, keep_focus_events_months, anonymise_disabled_after_months)
  ON app.organization_settings TO quizzivy_maintenance;
GRANT SELECT (id, assignment_id, student_id, status, submitted_at, deadline_at)
  ON app.attempts TO quizzivy_maintenance;
GRANT SELECT (id, created_by, published_at, closed_at, closes_at)
  ON app.assignments TO quizzivy_maintenance;
GRANT SELECT (attempt_id, question_id, payload), UPDATE (payload)
  ON app.attempt_answers TO quizzivy_maintenance;
GRANT SELECT (id, attempt_id, received_at), DELETE
  ON app.attempt_events TO quizzivy_maintenance;
GRANT INSERT ON app.audit_log TO quizzivy_maintenance;

-- retention-sweep's accounts step and anonymize-user: the account and what
-- ties it to a person. The owner check reads the owner column of the tables a
-- teacher's content lives in.
GRANT SELECT (id, email, full_name, display_name, role_id, disabled_at),
      UPDATE (email, full_name, display_name, phone, avatar_key, password_hash, must_change_password, disabled_at)
  ON app.users TO quizzivy_maintenance;
GRANT SELECT ON app.student_like_roles TO quizzivy_maintenance;
GRANT SELECT (user_id), DELETE ON app.user_identities TO quizzivy_maintenance;
GRANT SELECT (user_id), DELETE ON app.refresh_tokens TO quizzivy_maintenance;
GRANT SELECT (user_id, used_at, revoked_at), UPDATE (revoked_at) ON app.set_password_tokens TO quizzivy_maintenance;
GRANT SELECT (teacher_id) ON app.classes TO quizzivy_maintenance;
GRANT SELECT (owner_id) ON app.tests TO quizzivy_maintenance;
GRANT SELECT (owner_id) ON app.questions TO quizzivy_maintenance;
GRANT SELECT (owner_id) ON app.question_groups TO quizzivy_maintenance;
GRANT SELECT (owner_id) ON app.media_assets TO quizzivy_maintenance;

-- UPDATE on the login columns is what anonymising needs, and it is also what a
-- leaked credential for this role would use to take an account over: a new
-- email, a known password hash, the account enabled again. So a trigger that
-- fires for this role alone refuses, with 42501, any update that does not leave
-- the row anonymised: the reserved address maintenance.anonymousEmail writes, no
-- password hash, no phone, photo or display name, and disabled. That is what the
-- sweep and anonymize-user write and all they write; full_name is a name, not a
-- credential, and is not pinned. The role can insert neither an identity nor a
-- set-password link, so an anonymised row cannot be reopened by it either.
--
-- SECURITY INVOKER, because the check reads only the new row; search_path is
-- pinned and every type is schema-qualified, as 00059 does for its definer
-- function, and EXECUTE is revoked from PUBLIC. The role holds no right to set
-- session_replication_role, the one way a session skips a trigger. Every other
-- role pays the WHEN test and nothing else.
-- +goose StatementBegin
CREATE FUNCTION app.users_maintenance_guard() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, pg_temp
AS $fn$
BEGIN
  IF NEW.email = 'anonymous-' || NEW.id::pg_catalog.text || '@anonymous.invalid'
     AND NEW.password_hash IS NULL
     AND NEW.must_change_password = false
     AND NEW.disabled_at IS NOT NULL
     AND NEW.phone IS NULL
     AND NEW.avatar_key IS NULL
     AND NEW.display_name IS NULL THEN
    RETURN NEW;
  END IF;
  RAISE EXCEPTION 'quizzivy_maintenance may only leave an account anonymised'
    USING ERRCODE = 'insufficient_privilege';
END;
$fn$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION app.users_maintenance_guard() FROM PUBLIC;

CREATE TRIGGER users_maintenance_guard BEFORE UPDATE ON app.users
  FOR EACH ROW WHEN (current_user = 'quizzivy_maintenance')
  EXECUTE FUNCTION app.users_maintenance_guard();

-- window-schedule, window-list, window-cancel and window-end (R1), with R4's
-- override close times.
GRANT SELECT, INSERT (starts_at, ends_at), UPDATE (ends_at, cancelled_at)
  ON app.maintenance_windows TO quizzivy_maintenance;
GRANT UPDATE (deadline_at) ON app.attempts TO quizzivy_maintenance;
GRANT UPDATE (closes_at) ON app.assignments TO quizzivy_maintenance;
GRANT SELECT (assignment_id, student_id, closes_at), UPDATE (closes_at)
  ON app.assignment_student_overrides TO quizzivy_maintenance;

-- rekey-join-codes (R2).
GRANT SELECT (id, class_id, revoked_at, code_ciphertext, key_id, lookup_scheme),
      UPDATE (code_hash, code_ciphertext, key_id)
  ON app.class_join_codes TO quizzivy_maintenance;

-- ai-credits and ai-reservations-sweep (D4, T-D4.31). The ledger is never
-- granted: no maintenance command writes or reads a usage row.
GRANT SELECT, INSERT, UPDATE ON app.ai_credit_periods TO quizzivy_maintenance;
GRANT SELECT, INSERT, UPDATE ON app.ai_teacher_credits TO quizzivy_maintenance;
GRANT SELECT, UPDATE ON app.ai_settings TO quizzivy_maintenance;
GRANT SELECT, UPDATE ON app.ai_credit_requests TO quizzivy_maintenance;
GRANT SELECT, DELETE ON app.ai_reservations TO quizzivy_maintenance;
GRANT SELECT (id, user_id, kind, params, target, dedupe_key), INSERT (user_id, kind, params, target, dedupe_key),
      UPDATE (kind, params, target, read_at), DELETE
  ON app.notifications TO quizzivy_maintenance;
GRANT SELECT (user_id, event, in_app) ON app.notification_preferences TO quizzivy_maintenance;
GRANT SELECT ON app.roles, app.role_permissions TO quizzivy_maintenance;

-- +goose Down

DROP TRIGGER users_maintenance_guard ON app.users;
DROP FUNCTION app.users_maintenance_guard();

REVOKE ALL ON ALL TABLES IN SCHEMA app FROM quizzivy_maintenance;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA app FROM quizzivy_maintenance;
REVOKE USAGE ON SCHEMA app FROM quizzivy_maintenance;
REVOKE USAGE ON SCHEMA public FROM quizzivy_maintenance;
