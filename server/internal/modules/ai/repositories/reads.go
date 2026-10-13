package repositories

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/db"
)

const usageOfTeacher = `
	SELECT coalesce(t.allowance_credits,
	                CASE WHEN prior.custom THEN prior.allowance_credits - prior.granted_credits
	                     ELSE s.default_allowance_credits END),
	       coalesce(t.used_credits, 0), coalesce(t.reserved_credits, 0),
	       coalesce(t.paused, prior.paused, false),
	       coalesce(c.pool_credits + c.added_credits - c.used_credits - c.reserved_credits, s.pool_credits),
	       s.on_teacher_limit
	  FROM app.ai_settings s
	  LEFT JOIN app.ai_credit_periods c ON c.month = $1::date
	  LEFT JOIN app.ai_teacher_credits t ON t.month = $1::date AND t.user_id = $2::uuid
	  LEFT JOIN LATERAL (
	    SELECT p.allowance_credits, p.granted_credits, p.custom, p.paused
	      FROM app.ai_teacher_credits p
	     WHERE t.user_id IS NULL AND p.user_id = $2::uuid AND p.month < $1::date
	     ORDER BY p.month DESC LIMIT 1) prior ON true`

// Usage reads one teacher's month without creating a row: the figures of a teacher who has used nothing are the allowance and pause a first use would be given.
func (p *Postgres) Usage(ctx context.Context, month time.Time, userID string) (domain.MonthUsage, error) {
	out := domain.MonthUsage{Month: month}
	var reserved, poolLeft int64
	var limit string
	if err := p.QueryRow(ctx, usageOfTeacher, month, userID).
		Scan(&out.Allowance, &out.Used, &reserved, &out.Paused, &poolLeft, &limit); err != nil {
		return domain.MonthUsage{}, fmt.Errorf("ai: read the teacher's month: %w", err)
	}
	out.Left = max(min(out.Allowance-out.Used-reserved, poolLeft), 0)
	out.Requests = domain.Limit(limit) == domain.LimitRequest
	pending, err := pendingOf(ctx, p, month, userID)
	if err != nil {
		return domain.MonthUsage{}, err
	}
	out.Pending = pending
	if out.Categories, err = categorySums(ctx, p, month, userID); err != nil {
		return domain.MonthUsage{}, err
	}
	return out, nil
}

const requestColumns = `
	r.id::text, r.user_id::text, r.month, r.amount_credits, r.reason, r.status,
	coalesce(r.decided_by::text, ''), r.decided_at, r.created_at`

func scanRequest(row pgx.Row, extra ...any) (domain.Request, error) {
	var r domain.Request
	var status string
	dest := append([]any{&r.ID, &r.UserID, &r.Month, &r.Amount, &r.Reason, &status, &r.DecidedBy, &r.DecidedAt, &r.CreatedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return domain.Request{}, err
	}
	r.Status = domain.RequestStatus(status)
	return r, nil
}

func pendingOf(ctx context.Context, q db.Querier, month time.Time, userID string) (*domain.Request, error) {
	rows, err := db.QueryMany(ctx, q, `
		SELECT `+requestColumns+` FROM app.ai_credit_requests r
		 WHERE r.user_id = $1::uuid AND r.month = $2::date AND r.status = 'pending'`,
		[]any{userID, month}, func(rows pgx.Rows) (domain.Request, error) { return scanRequest(rows) })
	if err != nil {
		return nil, fmt.Errorf("ai: read the teacher's request: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

const poolOfMonth = `
	SELECT coalesce(c.pool_credits, s.pool_credits), coalesce(c.added_credits, 0),
	       coalesce(c.used_credits, 0), coalesce(c.reserved_credits, 0)
	  FROM app.ai_settings s
	  LEFT JOIN app.ai_credit_periods c ON c.month = $1::date`

const teachersOfMonth = `
	WITH eligible AS (
	  SELECT u.id FROM app.users u JOIN app.roles r ON r.id = u.role_id
	   WHERE u.disabled_at IS NULL
	     AND (r.builtin_key = 'admin'
	          OR EXISTS (SELECT 1 FROM app.role_permissions rp WHERE rp.role_id = r.id AND rp.permission_key = 'ai.use'))
	  UNION
	  SELECT c.user_id FROM app.ai_teacher_credits c WHERE c.month = $1::date
	)
	SELECT u.id::text, u.full_name, u.email,
	       coalesce(t.allowance_credits,
	                CASE WHEN prior.custom THEN prior.allowance_credits - prior.granted_credits
	                     ELSE s.default_allowance_credits END),
	       coalesce(t.granted_credits, 0), coalesce(t.used_credits, 0), coalesce(t.reserved_credits, 0),
	       coalesce(t.custom, prior.custom, false), coalesce(t.paused, prior.paused, false)
	  FROM eligible e
	  JOIN app.users u ON u.id = e.id
	  CROSS JOIN app.ai_settings s
	  LEFT JOIN app.ai_teacher_credits t ON t.month = $1::date AND t.user_id = u.id
	  LEFT JOIN LATERAL (
	    SELECT p.allowance_credits, p.granted_credits, p.custom, p.paused
	      FROM app.ai_teacher_credits p
	     WHERE t.user_id IS NULL AND p.user_id = u.id AND p.month < $1::date
	     ORDER BY p.month DESC LIMIT 1) prior ON true
	 ORDER BY u.full_name, u.id`

const pendingOfMonth = `
	SELECT ` + requestColumns + `, u.full_name
	  FROM app.ai_credit_requests r JOIN app.users u ON u.id = r.user_id
	 WHERE r.status = 'pending' AND r.month = $1::date
	 ORDER BY r.created_at, r.id`

// Admin reads the month for the Admin page: the pool, every teacher who holds ai.use or used AI that month, the requests waiting and the rules. It creates no row, and the Admin and the teachers are one list because an Admin spends from the same pool. The teachers are ordered by how much of their allowance they have used, then by name.
func (p *Postgres) Admin(ctx context.Context, month time.Time) (domain.AdminCredits, error) {
	rules, err := p.Rules(ctx)
	if err != nil {
		return domain.AdminCredits{}, err
	}
	out := domain.AdminCredits{Month: month, Rules: rules}
	if err := p.QueryRow(ctx, poolOfMonth, month).Scan(&out.Pool.Plan, &out.Pool.Added, &out.Pool.Used, &out.Pool.Reserved); err != nil {
		return domain.AdminCredits{}, fmt.Errorf("ai: read the pool: %w", err)
	}
	teachers, err := db.QueryMany(ctx, p, teachersOfMonth, []any{month}, func(rows pgx.Rows) (domain.TeacherMonth, error) {
		var t domain.TeacherMonth
		err := rows.Scan(&t.UserID, &t.Name, &t.Email, &t.Allowance, &t.Granted, &t.Used, &t.Reserved, &t.Custom, &t.Paused)
		return t, err
	})
	if err != nil {
		return domain.AdminCredits{}, fmt.Errorf("ai: read the teachers: %w", err)
	}
	byID := make(map[string]domain.TeacherMonth, len(teachers))
	for i := range teachers {
		teachers[i].Status = domain.StatusOf(teachers[i].Used, teachers[i].Allowance, rules.WarnPercent, teachers[i].Paused)
		byID[teachers[i].UserID] = teachers[i]
		out.Allowances += teachers[i].Allowance
		if teachers[i].Used > 0 {
			out.Active++
		} else {
			out.Idle++
		}
	}
	slices.SortStableFunc(teachers, busierFirst)
	out.Teachers = teachers
	out.Requests, err = db.QueryMany(ctx, p, pendingOfMonth, []any{month}, func(rows pgx.Rows) (domain.PendingRequest, error) {
		var pending domain.PendingRequest
		request, err := scanRequest(rows, &pending.TeacherName)
		pending.Request = request
		return pending, err
	})
	if err != nil {
		return domain.AdminCredits{}, fmt.Errorf("ai: read the requests: %w", err)
	}
	for i := range out.Requests {
		standing := byID[out.Requests[i].UserID]
		out.Requests[i].Used, out.Requests[i].Allowance = standing.Used, standing.Allowance
	}
	return out, nil
}

func busierFirst(a, b domain.TeacherMonth) int {
	if c := cmp.Compare(b.Used*a.Allowance, a.Used*b.Allowance); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}
