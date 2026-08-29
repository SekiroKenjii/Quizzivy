package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/audit"
)

// RotateOutcome classifies a presented refresh token. Rotate returns one of
// these rather than an error for the non-OK cases: "this token was reused" is a
// normal, expected result that the caller must act on, not a fault.
type RotateOutcome int

const (
	// RotateOK: the token was live and has been consumed; its successor exists.
	RotateOK RotateOutcome = iota
	// RotateUnknown: no such token. Never issued, or already pruned.
	RotateUnknown
	// RotateExpired: issued by us, but aged out. Not an attack.
	RotateExpired
	RotateReused
	RotateRevoked
	// RotateUserDisabled: the account was suspended after the token was issued.
	RotateUserDisabled
)

type RotateResult struct {
	Outcome  RotateOutcome
	User     User   // populated only on RotateOK
	FamilyID string // populated whenever the token was found
}

// Rotate consumes one refresh token and issues its successor, atomically.
//
// The outcome distinguishes a replay from an ordinary logout by replaced_by:
// set means the token was already exchanged, so the family is revoked; nil with
// revoked_at set means the user logged out.
func (s *Store) Rotate(ctx context.Context, tokenHash []byte, next RefreshTokenRecord, now time.Time) (RotateResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RotateResult{}, fmt.Errorf("begin rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const claim = `
		SELECT id::text, user_id::text, family_id::text, expires_at, revoked_at, replaced_by
		  FROM app.refresh_tokens
		 WHERE token_hash = $1
		   FOR UPDATE`

	var predecessorID, userID, familyID string
	var expiresAt time.Time
	var revokedAt *time.Time
	var replacedBy *string

	err = tx.QueryRow(ctx, claim, tokenHash).Scan(
		&predecessorID, &userID, &familyID, &expiresAt, &revokedAt, &replacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return RotateResult{Outcome: RotateUnknown}, nil
	}
	if err != nil {
		return RotateResult{}, fmt.Errorf("claim refresh token: %w", err)
	}
	if revokedAt != nil && replacedBy == nil {
		return RotateResult{Outcome: RotateRevoked, FamilyID: familyID}, nil
	}
	if revokedAt != nil {
		if err := revokeFamily(ctx, tx, familyID, now); err != nil {
			return RotateResult{}, err
		}
		if err := audit.Write(ctx, tx, audit.Entry{
			ActorUserID: &userID,
			Action:      "refresh_token.reuse_detected",
			Entity:      "refresh_token_family",
			EntityID:    &familyID,
			OccurredAt:  now,
			IP:          next.IP,
			UserAgent:   next.UserAgent,
		}); err != nil {
			return RotateResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return RotateResult{}, fmt.Errorf("commit family revocation: %w", err)
		}
		return RotateResult{Outcome: RotateReused, FamilyID: familyID}, nil
	}
	if !expiresAt.After(now) {
		return RotateResult{Outcome: RotateExpired, FamilyID: familyID}, nil
	}

	user, err := scanUser(tx.QueryRow(ctx, userProjection+`
		 WHERE u.id = $1
		 GROUP BY u.id`, userID))
	if err != nil {
		return RotateResult{}, fmt.Errorf("load token owner: %w", err)
	}
	if user.Disabled() {
		if err := revokeFamily(ctx, tx, familyID, now); err != nil {
			return RotateResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return RotateResult{}, fmt.Errorf("commit disabled-user revocation: %w", err)
		}
		return RotateResult{Outcome: RotateUserDisabled, FamilyID: familyID}, nil
	}

	const issue = `
		INSERT INTO app.refresh_tokens
		       (user_id, family_id, token_hash, issued_at, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var successorID string
	if err := tx.QueryRow(ctx, issue,
		userID, familyID, next.TokenHash, next.IssuedAt, next.ExpiresAt, next.UserAgent, next.IP,
	).Scan(&successorID); err != nil {
		return RotateResult{}, fmt.Errorf("issue successor token: %w", err)
	}
	const consume = `
		UPDATE app.refresh_tokens
		   SET revoked_at = $2, replaced_by = $3
		 WHERE id = $1`
	if _, err := tx.Exec(ctx, consume, predecessorID, now, successorID); err != nil {
		return RotateResult{}, fmt.Errorf("consume predecessor token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RotateResult{}, fmt.Errorf("commit rotation: %w", err)
	}
	return RotateResult{Outcome: RotateOK, User: user, FamilyID: familyID}, nil
}

// RevokeFamilyByToken ends every session descended from the same login. It is
// what logout does, and it does not care whether the presented token is still
// live: revoking an already-revoked family is a no-op, and refusing to would
// make logout fail exactly when the user needs it most.
//
// Returns the family id, or ErrRefreshTokenNotFound if the token is unknown.
func (s *Store) RevokeFamilyByToken(ctx context.Context, tokenHash []byte, now time.Time) (string, error) {
	const q = `
		WITH presented AS (
			SELECT family_id FROM app.refresh_tokens WHERE token_hash = $1
		)
		UPDATE app.refresh_tokens t
		   SET revoked_at = $2
		  FROM presented p
		 WHERE t.family_id = p.family_id AND t.revoked_at IS NULL
		RETURNING t.family_id::text`

	rows, err := s.pool.Query(ctx, q, tokenHash, now)
	if err != nil {
		return "", fmt.Errorf("revoke family: %w", err)
	}
	defer rows.Close()

	var familyID string
	for rows.Next() {
		if err := rows.Scan(&familyID); err != nil {
			return "", fmt.Errorf("revoke family: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("revoke family: %w", err)
	}
	if familyID != "" {
		return familyID, nil
	}
	const lookup = `SELECT family_id::text FROM app.refresh_tokens WHERE token_hash = $1`
	err = s.pool.QueryRow(ctx, lookup, tokenHash).Scan(&familyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrRefreshTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("revoke family: %w", err)
	}
	return familyID, nil
}

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

func revokeFamily(ctx context.Context, tx pgx.Tx, familyID string, now time.Time) error {
	const q = `
		UPDATE app.refresh_tokens
		   SET revoked_at = $2
		 WHERE family_id = $1 AND revoked_at IS NULL`
	if _, err := tx.Exec(ctx, q, familyID, now); err != nil {
		return fmt.Errorf("revoke token family %s: %w", familyID, err)
	}
	return nil
}

// DeleteExpired prunes refresh-token families whose every member has expired.
//
// By family, not by row: replaced_by is ON DELETE SET NULL, and a NULL
// replaced_by is how Rotate tells a logout from a replay. Pruning row by row
// would turn a detected reuse into an ordinary logout weeks later.
func (s *Store) DeleteExpired(ctx context.Context, before time.Time) (int64, error) {
	const q = `
		DELETE FROM app.refresh_tokens
		 WHERE family_id IN (
		       SELECT family_id
		         FROM app.refresh_tokens
		        GROUP BY family_id
		       HAVING max(expires_at) < $1)`
	tag, err := s.pool.Exec(ctx, q, before)
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ChangePasswordRecord is what the store needs to swap a password and prune the
// sessions that the old one authorised.
type ChangePasswordRecord struct {
	UserID        string
	NewHash       string
	KeepTokenHash []byte
	Now           time.Time
	IP            *string
	UserAgent     *string
}

// ChangePassword swaps the hash, clears must_change_password, and revokes every
// refresh family except the caller's -- all in one transaction.
//
// One transaction because the halves are useless apart. A password changed
// without the revocation leaves a stolen session alive under a password its
// holder no longer knows, which is the whole reason the user changed it.
func (s *Store) ChangePassword(ctx context.Context, in ChangePasswordRecord) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password change: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var keepFamily *string
	if len(in.KeepTokenHash) > 0 {
		var family string
		err := tx.QueryRow(ctx,
			`SELECT family_id::text FROM app.refresh_tokens
			  WHERE token_hash = $1 AND user_id = $2`,
			in.KeepTokenHash, in.UserID).Scan(&family)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return fmt.Errorf("resolve surviving family: %w", err)
		default:
			keepFamily = &family
		}
	}

	const setPassword = `
		UPDATE app.users
		   SET password_hash = $2, must_change_password = false
		 WHERE id = $1`
	tag, err := tx.Exec(ctx, setPassword, in.UserID, in.NewHash)
	if err != nil {
		return fmt.Errorf("set password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	const revokeOthers = `
		UPDATE app.refresh_tokens
		   SET revoked_at = $3
		 WHERE user_id = $1
		   AND revoked_at IS NULL
		   AND ($2::uuid IS NULL OR family_id <> $2)`
	if _, err := tx.Exec(ctx, revokeOthers, in.UserID, keepFamily, in.Now); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.UserID,
		Action:      "user.password_changed",
		Entity:      "user",
		EntityID:    &in.UserID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password change: %w", err)
	}
	return nil
}
