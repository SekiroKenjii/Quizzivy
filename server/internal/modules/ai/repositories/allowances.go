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

const findTeacher = `
	SELECT u.full_name, u.email FROM app.users u
	 WHERE u.id = $1::uuid AND u.disabled_at IS NULL
	   AND NOT EXISTS (SELECT 1 FROM app.student_like_roles s WHERE s.id = u.role_id)`

func findTeacherNames(ctx context.Context, q db.Querier, userID string) (name, email string, err error) {
	err = q.QueryRow(ctx, findTeacher, userID).Scan(&name, &email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", domain.ErrNotATeacher
	}
	if err != nil {
		return "", "", fmt.Errorf("ai: find the teacher: %w", err)
	}
	return name, email, nil
}

const setAllowance = `
	WITH changed AS (
	  UPDATE app.ai_teacher_credits t
	     SET custom = coalesce($3::boolean, t.custom),
	         paused = coalesce($5::boolean, t.paused),
	         allowance_credits = CASE
	           WHEN $3::boolean IS NULL THEN t.allowance_credits
	           WHEN $3::boolean THEN $4::bigint + t.granted_credits
	           ELSE s.default_allowance_credits + t.granted_credits END
	    FROM app.ai_settings s
	   WHERE t.month = $1::date AND t.user_id = $2::uuid
	  RETURNING old.custom AS was_custom, old.paused AS was_paused, old.allowance_credits AS was_allowance,
	            new.custom, new.paused, new.allowance_credits, new.granted_credits, new.used_credits, new.reserved_credits
	), logged AS (
	  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
	  SELECT $6::uuid, 'ai.allowance.updated', 'ai_teacher_credits', $2::uuid, $7::inet, $8,
	         jsonb_build_object(
	           'month', $1::date,
	           'before', jsonb_build_object('custom', was_custom, 'paused', was_paused, 'allowanceCredits', was_allowance),
	           'after', jsonb_build_object('custom', custom, 'paused', paused, 'allowanceCredits', allowance_credits)
	         ) || $9::jsonb
	    FROM changed
	)
	SELECT custom, paused, allowance_credits, granted_credits, used_credits, reserved_credits FROM changed`

// SetAllowance changes one teacher's allowance or pause for the month in one transaction. It creates the month's rows first if this is the teacher's first, so an Admin can set an allowance before the teacher has used AI, and then writes the change and its audit row in a single statement. A custom allowance is the number given plus what approved requests added this month; switching back to the default is the default plus the same. A figure below what the teacher has used is refused, not raised.
func (p *Postgres) SetAllowance(ctx context.Context, in domain.SetAllowanceInput) (domain.TeacherMonth, error) {
	details, err := detailsOf(in.By)
	if err != nil {
		return domain.TeacherMonth{}, err
	}
	out := domain.TeacherMonth{UserID: in.UserID}
	err = p.InTx(ctx, "ai: set allowance", func(tx pgx.Tx) error {
		name, email, err := findTeacherNames(ctx, tx, in.UserID)
		if err != nil {
			return err
		}
		out.Name, out.Email = name, email
		if err := ensureMonth(ctx, tx, in.Month, in.UserID); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, setAllowance, in.Month, in.UserID, in.Custom, in.Credits, in.Paused,
			opt.String(in.By.UserID), opt.String(in.By.IP), opt.String(in.By.UserAgent), details).
			Scan(&out.Custom, &out.Paused, &out.Allowance, &out.Granted, &out.Used, &out.Reserved)
		if db.IsCheckViolation(err, allowanceConstraint) {
			return domain.ErrAllowanceBelowUse
		}
		if err != nil {
			return fmt.Errorf("ai: write the allowance: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.TeacherMonth{}, err
	}
	return out, nil
}

const addCredits = `
	WITH changed AS (
	  UPDATE app.ai_credit_periods SET added_credits = added_credits + $2::bigint
	   WHERE month = $1::date
	  RETURNING old.added_credits AS was_added, new.pool_credits, new.added_credits, new.used_credits, new.reserved_credits
	), logged AS (
	  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
	  SELECT $3::uuid, 'ai.credits.added', 'ai_credit_periods', NULL::uuid, $4::inet, $5,
	         jsonb_build_object('month', $1::date, 'credits', $2::bigint, 'addedBefore', was_added, 'addedAfter', added_credits) || $6::jsonb
	    FROM changed
	)
	SELECT pool_credits, added_credits, used_credits, reserved_credits FROM changed`

// AddCredits adds to the month's pool in one transaction that creates the pool if the month has not been used yet and writes the audit row, which names the Admin who approved the extra, in the same statement.
func (p *Postgres) AddCredits(ctx context.Context, in domain.AddCreditsInput) (domain.Pool, error) {
	details, err := detailsOf(in.By)
	if err != nil {
		return domain.Pool{}, err
	}
	var out domain.Pool
	err = p.InTx(ctx, "ai: add credits", func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, insertPeriod, in.Month); err != nil {
			return fmt.Errorf("ai: create the month's pool: %w", err)
		}
		if err := tx.QueryRow(ctx, addCredits, in.Month, in.Credits,
			opt.String(in.By.UserID), opt.String(in.By.IP), opt.String(in.By.UserAgent), details).
			Scan(&out.Plan, &out.Added, &out.Used, &out.Reserved); err != nil {
			return fmt.Errorf("ai: write the addition: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Pool{}, err
	}
	return out, nil
}

const lockRules = `
	SELECT pool_credits, default_allowance_credits, warn_percent, on_teacher_limit
	  FROM app.ai_settings FOR UPDATE`

const lockMonthsTeachers = `
	SELECT 1 FROM app.ai_teacher_credits WHERE month = $1::date AND NOT custom ORDER BY user_id FOR NO KEY UPDATE`

const setRules = `
	WITH changed AS (
	  UPDATE app.ai_settings
	     SET default_allowance_credits = $1::bigint, warn_percent = $2::smallint, on_teacher_limit = $3
	  RETURNING old.default_allowance_credits AS was_default, old.warn_percent AS was_warn, old.on_teacher_limit AS was_limit,
	            new.pool_credits, new.default_allowance_credits, new.warn_percent, new.on_teacher_limit
	), rewritten AS (
	  UPDATE app.ai_teacher_credits t
	     SET allowance_credits = greatest(c.default_allowance_credits + t.granted_credits, t.used_credits + t.reserved_credits)
	    FROM changed c
	   WHERE t.month = $4::date AND NOT t.custom
	     AND c.default_allowance_credits <> c.was_default
	     AND t.allowance_credits <> greatest(c.default_allowance_credits + t.granted_credits, t.used_credits + t.reserved_credits)
	  RETURNING t.user_id
	), logged AS (
	  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, ip, user_agent, diff)
	  SELECT $5::uuid, 'ai.rules.updated', 'ai_settings', NULL::uuid, $6::inet, $7,
	         jsonb_build_object(
	           'month', $4::date,
	           'before', jsonb_build_object('defaultAllowanceCredits', was_default, 'warnPercent', was_warn, 'onTeacherLimit', was_limit),
	           'after', jsonb_build_object('defaultAllowanceCredits', default_allowance_credits, 'warnPercent', warn_percent, 'onTeacherLimit', on_teacher_limit),
	           'rewritten', (SELECT count(*) FROM rewritten)
	         ) || $8::jsonb
	    FROM changed
	)
	SELECT pool_credits, default_allowance_credits, warn_percent, on_teacher_limit, (SELECT count(*) FROM rewritten)::bigint
	  FROM changed`

// SetRules changes the centre's rules in one transaction that locks the settings row, so two changes queue. When the default allowance changes it locks the month's rows that are not custom in key order, as the sweep does, and rewrites the month's allowances that are not custom to the new default plus what approved requests added, never below what a teacher has already used or holds, so lowering the default cannot break a constraint or take back a credit that was spent. The audit row records how many allowances moved.
func (p *Postgres) SetRules(ctx context.Context, in domain.SetRulesInput) (domain.RulesChange, error) {
	details, err := detailsOf(in.By)
	if err != nil {
		return domain.RulesChange{}, err
	}
	var out domain.RulesChange
	err = p.InTx(ctx, "ai: set rules", func(tx pgx.Tx) error {
		next, err := readRules(ctx, tx, lockRules)
		if err != nil {
			return err
		}
		next = next.Changed(in)
		if in.DefaultAllowance != nil {
			if _, err := tx.Exec(ctx, lockMonthsTeachers, in.Month); err != nil {
				return fmt.Errorf("ai: lock the month's allowances: %w", err)
			}
		}
		var limit string
		if err := tx.QueryRow(ctx, setRules, next.DefaultAllowance, next.WarnPercent, string(next.OnTeacherLimit), in.Month,
			opt.String(in.By.UserID), opt.String(in.By.IP), opt.String(in.By.UserAgent), details).
			Scan(&out.Rules.PoolCredits, &out.Rules.DefaultAllowance, &out.Rules.WarnPercent, &limit, &out.Rewritten); err != nil {
			return fmt.Errorf("ai: write the rules: %w", err)
		}
		out.Rules.OnTeacherLimit = domain.Limit(limit)
		return nil
	})
	if err != nil {
		return domain.RulesChange{}, err
	}
	return out, nil
}

func readRules(ctx context.Context, q db.Querier, sql string) (domain.Rules, error) {
	var rules domain.Rules
	var limit string
	if err := q.QueryRow(ctx, sql).Scan(&rules.PoolCredits, &rules.DefaultAllowance, &rules.WarnPercent, &limit); err != nil {
		return domain.Rules{}, fmt.Errorf("ai: read the rules: %w", err)
	}
	rules.OnTeacherLimit = domain.Limit(limit)
	return rules, nil
}

// Rules reads the centre's rules.
func (p *Postgres) Rules(ctx context.Context) (domain.Rules, error) {
	return readRules(ctx, p, `
		SELECT pool_credits, default_allowance_credits, warn_percent, on_teacher_limit FROM app.ai_settings`)
}
