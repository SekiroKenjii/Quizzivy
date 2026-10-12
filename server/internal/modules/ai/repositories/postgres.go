// Package repositories stores the AI assistant's pool, allowances, reservations, ledger and requests in PostgreSQL. Every statement that moves a credit takes the month's row before the teacher's, so concurrent calls queue in one order and cannot deadlock; the tables' constraints are what refuse an overspend.
package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/db"
)

// Postgres is the Repository over the app schema.
type Postgres struct{ db.Repository }

// NewPostgres returns the repository over dbx.
func NewPostgres(dbx db.Context) *Postgres { return &Postgres{Repository: db.NewRepository(dbx)} }

var _ domain.Repository = (*Postgres)(nil)

const (
	poolConstraint      = "ai_credit_periods_within_pool"
	allowanceConstraint = "ai_teacher_credits_within_allowance"
)

const insertPeriod = `
	INSERT INTO app.ai_credit_periods (month, pool_credits)
	SELECT $1::date, s.pool_credits FROM app.ai_settings s
	ON CONFLICT (month) DO NOTHING`

const lockPeriod = `SELECT 1 FROM app.ai_credit_periods WHERE month = $1::date FOR NO KEY UPDATE`

const insertTeacher = `
	INSERT INTO app.ai_teacher_credits (month, user_id, allowance_credits, custom, paused)
	SELECT $1::date, $2::uuid,
	       CASE WHEN prior.custom THEN prior.allowance_credits - prior.granted_credits
	            ELSE s.default_allowance_credits END,
	       coalesce(prior.custom, false), coalesce(prior.paused, false)
	  FROM app.ai_settings s
	  LEFT JOIN LATERAL (
	    SELECT t.allowance_credits, t.granted_credits, t.custom, t.paused
	      FROM app.ai_teacher_credits t
	     WHERE t.user_id = $2::uuid AND t.month < $1::date
	     ORDER BY t.month DESC LIMIT 1) prior ON true
	   FOR SHARE OF s
	ON CONFLICT (month, user_id) DO NOTHING`

const lockTeacher = `
	SELECT paused FROM app.ai_teacher_credits
	 WHERE month = $1::date AND user_id = $2::uuid FOR NO KEY UPDATE`

const carriedPause = `
	SELECT coalesce((SELECT t.paused FROM app.ai_teacher_credits t
	                  WHERE t.user_id = $2::uuid AND t.month <= $1::date
	                  ORDER BY t.month DESC LIMIT 1), false)`

func ensureMonth(ctx context.Context, q db.Querier, month time.Time, userID string) error {
	if _, err := q.Exec(ctx, insertPeriod, month); err != nil {
		return fmt.Errorf("ai: create the month's pool: %w", err)
	}
	if _, err := q.Exec(ctx, insertTeacher, month, userID); err != nil {
		return fmt.Errorf("ai: create the teacher's month: %w", err)
	}
	return nil
}

func lockMonth(ctx context.Context, q db.Querier, month time.Time, userID string) (paused bool, err error) {
	if _, err := q.Exec(ctx, lockPeriod, month); err != nil {
		return false, fmt.Errorf("ai: lock the month's pool: %w", err)
	}
	if err := q.QueryRow(ctx, lockTeacher, month, userID).Scan(&paused); err != nil {
		return false, fmt.Errorf("ai: lock the teacher's month: %w", err)
	}
	return paused, nil
}

type owner struct {
	month  time.Time
	userID string
}

func scanOwner(rows pgx.Rows) (owner, error) {
	var o owner
	err := rows.Scan(&o.month, &o.userID)
	return o, err
}

func detailsOf(by domain.Author) ([]byte, error) {
	details := map[string]any{}
	if by.System() {
		details["reason"] = "maintenance"
		details["operator"] = by.Operator
	}
	return json.Marshal(details)
}
