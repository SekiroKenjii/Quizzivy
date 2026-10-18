package maintenance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/platform/db"
)

const (
	studentPlaceholder = "Học viên đã ẩn danh"
	userPlaceholder    = "Người dùng đã ẩn danh"
)

const anonymousEmail = `'anonymous-' || u.id::text || '@anonymous.invalid'`

const notAnonymous = `u.email <> ` + anonymousEmail

const ownsContent = `(EXISTS (SELECT 1 FROM app.classes c WHERE c.teacher_id = u.id)
  OR EXISTS (SELECT 1 FROM app.tests t WHERE t.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.questions q WHERE q.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.question_groups g WHERE g.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.media_assets m WHERE m.owner_id = u.id)
  OR EXISTS (SELECT 1 FROM app.assignments s WHERE s.created_by = u.id))`

const hasAttemptInProgress = `EXISTS (SELECT 1 FROM app.attempts p WHERE p.student_id = u.id AND p.status = 'in_progress')`

const anonymiseUsers = `
	WITH targets AS (
	  SELECT u.id, u.full_name, u.display_name,
	         EXISTS (SELECT 1 FROM app.student_like_roles s WHERE s.id = u.role_id) AS student
	    FROM app.users u
	   WHERE u.id = ANY($1::uuid[]) AND ` + notAnonymous + `
	), names AS MATERIALIZED (
	  SELECT DISTINCT n AS name
	    FROM targets t CROSS JOIN LATERAL unnest(ARRAY[t.full_name, t.display_name]) AS n
	   WHERE n IS NOT NULL AND n <> ''
	), notices AS (
	  DELETE FROM app.notifications n
	   WHERE n.id IN (
	     SELECT m.id
	       FROM app.notifications m
	      CROSS JOIN LATERAL jsonb_path_query(m.params, 'strict $.** ? (@.type() == "string")') AS v
	       JOIN names ON names.name = (v #>> '{}'))
	  RETURNING 1
	), identities AS (
	  DELETE FROM app.user_identities WHERE user_id IN (SELECT id FROM targets)
	), sessions AS (
	  DELETE FROM app.refresh_tokens WHERE user_id IN (SELECT id FROM targets)
	), links AS (
	  UPDATE app.set_password_tokens SET revoked_at = now()
	   WHERE user_id IN (SELECT id FROM targets) AND used_at IS NULL AND revoked_at IS NULL
	), changed AS (
	  UPDATE app.users u
	     SET email = ` + anonymousEmail + `,
	         full_name = CASE WHEN t.student THEN $2::text ELSE $3::text END,
	         display_name = NULL, phone = NULL, avatar_key = NULL,
	         password_hash = NULL, must_change_password = false,
	         disabled_at = coalesce(u.disabled_at, now())
	    FROM targets t
	   WHERE u.id = t.id
	  RETURNING u.id
	), audited AS (
	  INSERT INTO app.audit_log (action, entity, entity_id, diff)
	  SELECT 'user.anonymized', 'users', id,
	         jsonb_strip_nulls(jsonb_build_object('structuredIdentityRemoved', true, 'reason', $4::text,
	           'cutoff', $5::timestamptz, 'databaseRole', current_user))
	    FROM changed
	)
	SELECT (SELECT count(*) FROM changed), (SELECT count(*) FROM notices)`

type anonymised struct {
	users   int64
	notices int64
}

func anonymise(ctx context.Context, tx pgx.Tx, ids []string, reason string, cutoff *time.Time) (anonymised, error) {
	var out anonymised
	err := tx.QueryRow(ctx, anonymiseUsers, ids, studentPlaceholder, userPlaceholder, reason, cutoff).Scan(&out.users, &out.notices)
	return out, err
}

// AnonymizationReport names the account an anonymize-user run handled without exposing its previous identity.
type AnonymizationReport struct {
	UserID  string `json:"userId"`
	Applied bool   `json:"applied"`
}

// AnonymizeUser removes an account's structured identity and login credentials on an explicit request, as the sweep does for a long-disabled account: its name becomes a placeholder, its email a reserved address, its photo reference, phone and password are cleared, its Google link and refresh tokens are removed and its unused set-password links are revoked.
// Notifications that carry its name are deleted. Attempts, memberships and the append-only audit trail stay for historical review. It refuses an account that owns classes or content, or has an attempt in progress.
func AnonymizeUser(ctx context.Context, conn db.Conn, userID string, apply bool) (AnonymizationReport, error) {
	out := AnonymizationReport{UserID: userID}
	if _, err := uuid.Parse(userID); err != nil {
		return out, errors.New("user ID must be a UUID")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var owns, busy, done bool
	err = tx.QueryRow(ctx, `SELECT `+ownsContent+`, `+hasAttemptInProgress+`, u.email = `+anonymousEmail+`
		 FROM app.users u WHERE u.id = $1 FOR UPDATE OF u`, userID).Scan(&owns, &busy, &done)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, errors.New("no account has that ID")
	}
	if err != nil {
		return out, fmt.Errorf("find account: %w", err)
	}
	if !done {
		if owns {
			return out, errors.New("the account owns classes or content: transfer it to another teacher before anonymizing")
		}
		if busy {
			return out, errors.New("finish or void the account's active attempts before anonymizing")
		}
	}
	if !apply {
		return out, nil
	}
	if _, err := anonymise(ctx, tx, []string{userID}, "request", nil); err != nil {
		return out, fmt.Errorf("anonymize account: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	out.Applied = true
	return out, nil
}
