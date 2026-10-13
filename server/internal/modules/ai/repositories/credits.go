package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/modules/ai/domain"
	"quizzivy/internal/platform/db"
)

// Reserve holds the credits in one transaction: it creates the month's pool and the teacher's month if this is their first use, locks the pool and then the teacher, and adds the credits to both reserved counts. A paused teacher is refused by an unlocked read before the transaction begins, and again under the lock, because an Admin may pause between the two. The teacher's allowance is checked before the pool, so a teacher who is out of credits is told so even when the pool is also out.
func (p *Postgres) Reserve(ctx context.Context, in domain.ReserveInput) (domain.Reservation, error) {
	var paused bool
	if err := p.QueryRow(ctx, carriedPause, in.Month, in.UserID).Scan(&paused); err != nil {
		return domain.Reservation{}, fmt.Errorf("ai: read the pause: %w", err)
	}
	if paused {
		return domain.Reservation{}, domain.ErrPaused
	}
	out := domain.Reservation{Month: in.Month, UserID: in.UserID, Feature: in.Feature, Credits: in.Credits}
	err := p.InTx(ctx, "ai: reserve", func(tx pgx.Tx) error {
		if err := ensureMonth(ctx, tx, in.Month, in.UserID); err != nil {
			return err
		}
		pausedNow, err := lockMonth(ctx, tx, in.Month, in.UserID)
		if err != nil {
			return err
		}
		if pausedNow {
			return domain.ErrPaused
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.ai_teacher_credits SET reserved_credits = reserved_credits + $3
			 WHERE month = $1::date AND user_id = $2::uuid`, in.Month, in.UserID, in.Credits); err != nil {
			return exhausted(err, allowanceConstraint, domain.ScopeTeacher)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.ai_credit_periods SET reserved_credits = reserved_credits + $2
			 WHERE month = $1::date`, in.Month, in.Credits); err != nil {
			return exhausted(err, poolConstraint, domain.ScopePool)
		}
		return tx.QueryRow(ctx, `
			INSERT INTO app.ai_reservations (month, user_id, feature, credits)
			VALUES ($1::date, $2::uuid, $3, $4) RETURNING id::text`,
			in.Month, in.UserID, string(in.Feature), in.Credits).Scan(&out.ID)
	})
	if err != nil {
		return domain.Reservation{}, err
	}
	return out, nil
}

func exhausted(err error, constraint string, scope domain.Scope) error {
	if db.IsCheckViolation(err, constraint) {
		return &domain.Exhausted{Scope: scope}
	}
	return fmt.Errorf("ai: hold the credits: %w", err)
}

const takeReservation = `
	DELETE FROM app.ai_reservations
	 WHERE id = $1::uuid AND month = $2::date AND user_id = $3::uuid
	RETURNING feature, credits`

type taken struct {
	feature domain.Feature
	credits int64
}

func takeHeld(ctx context.Context, tx pgx.Tx, r domain.Reservation) (taken, error) {
	if _, err := tx.Exec(ctx, lockPeriod, r.Month); err != nil {
		return taken{}, fmt.Errorf("ai: lock the month's pool: %w", err)
	}
	var locked bool
	err := tx.QueryRow(ctx, lockTeacher, r.Month, r.UserID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return taken{}, domain.ErrReservationGone
	}
	if err != nil {
		return taken{}, fmt.Errorf("ai: lock the teacher's month: %w", err)
	}
	var out taken
	var feature string
	err = tx.QueryRow(ctx, takeReservation, r.ID, r.Month, r.UserID).Scan(&feature, &out.credits)
	if errors.Is(err, pgx.ErrNoRows) {
		return taken{}, domain.ErrReservationGone
	}
	if err != nil {
		return taken{}, fmt.Errorf("ai: take the reservation: %w", err)
	}
	out.feature = domain.Feature(feature)
	return out, nil
}

const settleTeacher = `
	UPDATE app.ai_teacher_credits t
	   SET reserved_credits = t.reserved_credits - $3,
	       used_credits = t.used_credits + $4,
	       warned_at = CASE WHEN t.warned_at IS NULL AND $4 > 0
	                         AND (t.used_credits + $4) * 100 >= t.allowance_credits * s.warn_percent
	                        THEN now() ELSE t.warned_at END
	  FROM app.ai_settings s
	 WHERE t.month = $1::date AND t.user_id = $2::uuid
	RETURNING (old.warned_at IS NULL AND new.warned_at IS NOT NULL), new.used_credits, new.allowance_credits`

const settlePeriod = `
	UPDATE app.ai_credit_periods
	   SET reserved_credits = reserved_credits - $2, used_credits = used_credits + $3
	 WHERE month = $1::date`

// Settle ends a reservation in one transaction that locks the month's pool and the teacher before it takes the reservation, which is the order every other path uses. It moves what the call is charged from reserved to used and appends the ledger row. The charge is the metered credits capped at the reservation, so the update can only lower what the constraints add up and a call the provider has already been paid for is never refused; the ledger keeps the real token counts.
func (p *Postgres) Settle(ctx context.Context, s domain.Settlement) (domain.Settled, error) {
	var out domain.Settled
	err := p.InTx(ctx, "ai: settle", func(tx pgx.Tx) error {
		held, err := takeHeld(ctx, tx, s.Reservation)
		if err != nil {
			return err
		}
		out = domain.Settled{Charged: min(s.Metered, held.credits), Metered: s.Metered}
		var crossed bool
		var used, allowance int64
		if err := tx.QueryRow(ctx, settleTeacher, s.Reservation.Month, s.Reservation.UserID, held.credits, out.Charged).
			Scan(&crossed, &used, &allowance); err != nil {
			return fmt.Errorf("ai: charge the teacher: %w", err)
		}
		if _, err := tx.Exec(ctx, settlePeriod, s.Reservation.Month, held.credits, out.Charged); err != nil {
			return fmt.Errorf("ai: charge the pool: %w", err)
		}
		if err := appendLedger(ctx, tx, s, held.feature, out.Charged); err != nil {
			return err
		}
		if crossed {
			out.Warning = &domain.Warning{UserID: s.Reservation.UserID, Month: s.Reservation.Month, Used: used, Allowance: allowance}
		}
		return nil
	})
	if err != nil {
		return domain.Settled{}, err
	}
	return out, nil
}

// Release returns a reservation's credits, locking in the same order as Settle. It reports false when the reservation was not held, which a second release or a sweep that got there first makes true of every later call.
func (p *Postgres) Release(ctx context.Context, r domain.Reservation) (bool, error) {
	released := false
	err := p.InTx(ctx, "ai: release", func(tx pgx.Tx) error {
		held, err := takeHeld(ctx, tx, r)
		if errors.Is(err, domain.ErrReservationGone) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.ai_teacher_credits SET reserved_credits = reserved_credits - $3
			 WHERE month = $1::date AND user_id = $2::uuid`, r.Month, r.UserID, held.credits); err != nil {
			return fmt.Errorf("ai: return the teacher's credits: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE app.ai_credit_periods SET reserved_credits = reserved_credits - $2
			 WHERE month = $1::date`, r.Month, held.credits); err != nil {
			return fmt.Errorf("ai: return the pool's credits: %w", err)
		}
		released = true
		return nil
	})
	return released, err
}

const staleOwners = `
	SELECT DISTINCT month, user_id::text FROM app.ai_reservations
	 WHERE created_at < $1 ORDER BY month, user_id`

const lockStalePeriods = `
	SELECT 1 FROM app.ai_credit_periods WHERE month = ANY($1::date[]) ORDER BY month FOR NO KEY UPDATE`

const lockStaleTeachers = `
	SELECT 1 FROM app.ai_teacher_credits t
	  JOIN unnest($1::date[], $2::uuid[]) AS k (month, user_id) USING (month, user_id)
	 ORDER BY t.month, t.user_id FOR NO KEY UPDATE OF t`

const sweepStale = `
	WITH stale AS (
	  DELETE FROM app.ai_reservations WHERE created_at < $1 RETURNING month, user_id, credits
	), per_teacher AS (
	  SELECT month, user_id, sum(credits)::bigint AS credits FROM stale GROUP BY month, user_id
	), per_period AS (
	  SELECT month, sum(credits)::bigint AS credits FROM stale GROUP BY month
	), teachers AS (
	  UPDATE app.ai_teacher_credits t SET reserved_credits = t.reserved_credits - d.credits
	    FROM per_teacher d WHERE t.month = d.month AND t.user_id = d.user_id
	  RETURNING 1
	), periods AS (
	  UPDATE app.ai_credit_periods c SET reserved_credits = c.reserved_credits - d.credits
	    FROM per_period d WHERE c.month = d.month
	  RETURNING 1
	)
	SELECT (SELECT count(*) FROM stale), (SELECT coalesce(sum(credits), 0)::bigint FROM stale)`

// ReleaseStale returns the reservations created before cutoff. It finds whose they are, locks the pools and then the teachers in key order, and only then deletes and returns them in one statement, so it queues behind the calls it races in the order they queue themselves.
func (p *Postgres) ReleaseStale(ctx context.Context, cutoff time.Time) (domain.Released, error) {
	var out domain.Released
	err := p.InTx(ctx, "ai: sweep reservations", func(tx pgx.Tx) error {
		owners, err := db.QueryMany(ctx, tx, staleOwners, []any{cutoff}, scanOwner)
		if err != nil {
			return fmt.Errorf("ai: find stale reservations: %w", err)
		}
		if len(owners) == 0 {
			return nil
		}
		months, users := make([]time.Time, len(owners)), make([]string, len(owners))
		for i, o := range owners {
			months[i], users[i] = o.month, o.userID
		}
		if _, err := tx.Exec(ctx, lockStalePeriods, months); err != nil {
			return fmt.Errorf("ai: lock pools: %w", err)
		}
		if _, err := tx.Exec(ctx, lockStaleTeachers, months, users); err != nil {
			return fmt.Errorf("ai: lock teachers: %w", err)
		}
		return tx.QueryRow(ctx, sweepStale, cutoff).Scan(&out.Reservations, &out.Credits)
	})
	return out, err
}
