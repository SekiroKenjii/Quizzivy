package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/opt"
)

const pendingIndex = "ai_credit_requests_one_pending_idx"

const managersOfCredits = `
	SELECT u.id::text FROM app.users u JOIN app.roles r ON r.id = u.role_id
	 WHERE u.disabled_at IS NULL
	   AND (r.builtin_key = 'admin'
	        OR EXISTS (SELECT 1 FROM app.role_permissions rp WHERE rp.role_id = r.id AND rp.permission_key = 'ai.credits.manage'))
	 ORDER BY u.id`

// CreateRequest records a teacher's request in one transaction that reads the centre's rule under a share lock, so a rule change cannot land between the check and the insert. The partial unique index is what allows one pending request a month: a second insert answers ErrRequestPending. The result names every user who holds ai.credits.manage (the Admin role holds it by being the wildcard), so the caller can tell them.
func (p *Postgres) CreateRequest(ctx context.Context, in domain.CreateRequestInput) (domain.Filed, error) {
	var out domain.Filed
	err := p.InTx(ctx, "ai: file a request", func(tx pgx.Tx) error {
		name, _, err := findTeacherNames(ctx, tx, in.UserID)
		if err != nil {
			return err
		}
		var limit string
		if err := tx.QueryRow(ctx, `SELECT on_teacher_limit FROM app.ai_settings FOR SHARE`).Scan(&limit); err != nil {
			return fmt.Errorf("ai: read the rule: %w", err)
		}
		if domain.Limit(limit) != domain.LimitRequest {
			return domain.ErrRequestsOff
		}
		request, err := scanRequest(tx.QueryRow(ctx, `
			INSERT INTO app.ai_credit_requests AS r (user_id, month, amount_credits, reason)
			VALUES ($1::uuid, $2::date, $3, $4)
			RETURNING `+requestColumns, in.UserID, in.Month, in.Amount, in.Reason))
		if db.IsUniqueViolation(err, pendingIndex) {
			return domain.ErrRequestPending
		}
		if err != nil {
			return fmt.Errorf("ai: record the request: %w", err)
		}
		managers, err := db.QueryMany(ctx, tx, managersOfCredits, nil, func(rows pgx.Rows) (string, error) {
			var id string
			err := rows.Scan(&id)
			return id, err
		})
		if err != nil {
			return fmt.Errorf("ai: find who manages credits: %w", err)
		}
		out = domain.Filed{Request: request, TeacherName: name, Managers: managers}
		return nil
	})
	if err != nil {
		return domain.Filed{}, err
	}
	return out, nil
}

const lockRequest = `
	SELECT ` + requestColumns + `, u.full_name
	  FROM app.ai_credit_requests r JOIN app.users u ON u.id = r.user_id
	 WHERE r.id = $1::uuid FOR UPDATE OF r`

const decideRequest = `
	WITH decided AS (
	  UPDATE app.ai_credit_requests AS r
	     SET status = $2, decided_by = $3::uuid, decided_at = now()
	   WHERE r.id = $1::uuid AND r.status = 'pending'
	  RETURNING r.id, r.user_id, r.month, r.amount_credits, r.reason, r.status, r.decided_by, r.decided_at, r.created_at
	), raised AS (
	  UPDATE app.ai_teacher_credits t
	     SET allowance_credits = t.allowance_credits + d.amount_credits,
	         granted_credits = t.granted_credits + d.amount_credits
	    FROM decided d
	   WHERE d.status = 'approved' AND t.month = d.month AND t.user_id = d.user_id
	  RETURNING old.allowance_credits AS was_allowance, new.allowance_credits AS allowance
	), logged AS (
	  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
	  SELECT $3::uuid, 'ai.credit_request.' || d.status, 'ai_credit_requests', d.id, $4::inet, $5,
	         jsonb_build_object(
	           'userId', d.user_id, 'month', d.month, 'credits', d.amount_credits,
	           'allowanceBefore', (SELECT was_allowance FROM raised), 'allowanceAfter', (SELECT allowance FROM raised)
	         ) || $6::jsonb
	    FROM decided d
	)
	SELECT d.id::text, d.user_id::text, d.month, d.amount_credits, d.reason, d.status,
	       coalesce(d.decided_by::text, ''), d.decided_at, d.created_at,
	       coalesce((SELECT allowance FROM raised), 0)::bigint
	  FROM decided d`

func lockPending(ctx context.Context, tx pgx.Tx, in domain.DecideInput) (domain.Request, string, error) {
	var name string
	current, err := scanRequest(tx.QueryRow(ctx, lockRequest, in.ID), &name)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Request{}, "", domain.ErrRequestNotFound
	}
	if err != nil {
		return domain.Request{}, "", fmt.Errorf("ai: lock the request: %w", err)
	}
	if current.Status != domain.RequestPending {
		return domain.Request{}, "", domain.ErrRequestDecided
	}
	if in.Approve && !current.Month.Equal(in.Month) {
		return domain.Request{}, "", domain.ErrRequestExpired
	}
	return current, name, nil
}

// DecideRequest decides a request in one transaction. It locks the request row, so two Admins deciding the same request queue and the second is told it was decided, refuses an approval of an earlier month's request, creates the teacher's month when an approval needs it, and writes the status, the raised allowance and the audit row in one statement. An approval raises the allowance for the request's month by the amount and counts it as granted, which a change of the default rule leaves alone and a new month does not carry.
func (p *Postgres) DecideRequest(ctx context.Context, in domain.DecideInput) (domain.Decided, error) {
	details, err := detailsOf(in.By)
	if err != nil {
		return domain.Decided{}, err
	}
	status := domain.RequestDeclined
	if in.Approve {
		status = domain.RequestApproved
	}
	var out domain.Decided
	err = p.InTx(ctx, "ai: decide a request", func(tx pgx.Tx) error {
		current, name, err := lockPending(ctx, tx, in)
		if err != nil {
			return err
		}
		if in.Approve {
			if err := ensureMonth(ctx, tx, current.Month, current.UserID); err != nil {
				return err
			}
		}
		var allowance int64
		decided, err := scanRequest(tx.QueryRow(ctx, decideRequest, in.ID, string(status),
			opt.String(in.By.UserID), opt.String(in.By.IP), opt.String(in.By.UserAgent), details), &allowance)
		if err != nil {
			return fmt.Errorf("ai: write the decision: %w", err)
		}
		out = domain.Decided{Request: decided, TeacherName: name, Allowance: allowance}
		return nil
	})
	if err != nil {
		return domain.Decided{}, err
	}
	return out, nil
}
