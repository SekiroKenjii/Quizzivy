package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	// RotateReused: this token had already been rotated. §5.2 -- the family
	// is now revoked.
	RotateReused
	// RotateRevoked: revoked wholesale rather than rotated -- by a logout, or
	// by the cascade from someone else's reuse. Not this caller's doing.
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
// The whole operation runs in one transaction opened by SELECT ... FOR UPDATE
// on the presented token. That lock is what makes reuse detection correct under
// concurrency. Two simultaneous refreshes of the same token both reach the
// SELECT; one takes the row lock and proceeds, the other blocks. When the first
// commits, READ COMMITTED re-reads the row for the waiter, which now sees
// revoked_at set and is classified as a reuse.
//
// The alternative -- read, decide, then write -- is a read-then-write race in
// which both callers see a live token and both rotate it, leaving two valid
// successors in one family and reuse detection that never fires.
//
// `next` supplies the successor's token hash, expiry, and request metadata.
// Its UserID and FamilyID are ignored: both are inherited from the predecessor,
// because a rotation that could change either would not be a rotation.
func (s *Store) Rotate(ctx context.Context, tokenHash []byte, next RefreshTokenRecord, now time.Time) (RotateResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RotateResult{}, fmt.Errorf("begin rotation: %w", err)
	}
	// Safe after Commit: pgx treats rollback of a finished transaction as a
	// no-op, so this covers every early return without guarding each one.
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

	// A revoked token is not automatically a reused one, and the difference is
	// visible in replaced_by. Set means THIS token was consumed by a rotation,
	// so presenting it again is a replay. Null means it was revoked wholesale
	// -- by a logout, or by the cascade from someone else's replay -- which the
	// present caller did not do and must not be accused of.
	//
	// Collapsing the two would tell a student who simply logged out that
	// someone else had used their session.
	if revokedAt != nil && replacedBy == nil {
		return RotateResult{Outcome: RotateRevoked, FamilyID: familyID}, nil
	}

	// §5.2 reuse detection. Someone is presenting a token we already consumed:
	// either an attacker replaying a stolen copy, or the legitimate client
	// racing itself. We cannot tell the two apart, so we assume the worse one
	// and end every session in the lineage.
	if revokedAt != nil {
		if err := revokeFamily(ctx, tx, familyID, now); err != nil {
			return RotateResult{}, err
		}
		if err := insertAudit(ctx, tx, auditEntry{
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

	// Expiry is not reuse. The family is already dead -- this was its live
	// token -- so there is nothing to revoke and nothing to record.
	if !expiresAt.After(now) {
		return RotateResult{Outcome: RotateExpired, FamilyID: familyID}, nil
	}

	user, err := scanUser(tx.QueryRow(ctx, userProjection+`
		 WHERE u.id = $1
		 GROUP BY u.id`, userID))
	if err != nil {
		return RotateResult{}, fmt.Errorf("load token owner: %w", err)
	}

	// A refresh token outlives a suspension by up to 30 days. Re-reading the
	// user is what stops a disabled account from renewing itself indefinitely.
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

	// Consume the predecessor. Both columns are set in ONE update: replaced_by
	// carries a foreign key to the successor, so this cannot run before the
	// insert above, and splitting it into two updates would write two row
	// versions for no gain.
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

	// No rows updated: either the token is unknown, or its family was already
	// fully revoked. Those are different answers to the caller.
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

// DeleteExpired prunes rotation chains that can no longer authenticate
// anything. It deletes by FAMILY, and only once every token in that family has
// expired.
//
// Deleting individual expired rows looks equivalent and is not. replaced_by is
// ON DELETE SET NULL, and Rotate reads replaced_by to tell a token that was
// ROTATED (a replay -- revoke the family) from one revoked wholesale (a logout
// -- just refuse). Pruning a successor therefore nulls its predecessor's
// replaced_by and silently downgrades reuse detection on that predecessor to a
// plain rejection: the family stays alive and nothing is audited.
//
// Usually harmless, because a successor is issued later than its predecessor
// and so expires later, and both go in the same statement. It stops being
// harmless the moment REFRESH_TOKEN_TTL is REDUCED: tokens minted just after
// the change expire before their own predecessors, and the predecessors are
// left behind with a nulled link. Pruning whole families removes the ordering
// assumption instead of relying on it.
//
// This does not use refresh_tokens_expiry_idx -- the aggregate scans the table.
// At one family per login for fifty students that is not a cost worth shaping
// the correctness around.
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

// auditEntry is the subset of app.audit_log this package writes. A shared audit
// helper belongs in its own package once a second feature needs one; inventing
// it for a single caller would be guessing at that package's shape.
type auditEntry struct {
	ActorUserID *string
	Action      string
	Entity      string
	EntityID    *string
	OccurredAt  time.Time
	IP          *string
	UserAgent   *string
}

func insertAudit(ctx context.Context, tx pgx.Tx, e auditEntry) error {
	const q = `
		INSERT INTO app.audit_log
		       (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := tx.Exec(ctx, q,
		e.ActorUserID, e.Action, e.Entity, e.EntityID, e.OccurredAt, e.IP, e.UserAgent); err != nil {
		return fmt.Errorf("write audit entry %s: %w", e.Action, err)
	}
	return nil
}

// ChangePasswordRecord is what the store needs to swap a password and prune the
// sessions that the old one authorised.
type ChangePasswordRecord struct {
	UserID  string
	NewHash string
	// KeepTokenHash identifies the CALLER's session, whose family survives.
	// Nil revokes every family, including the caller's.
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

	// Resolve the surviving family inside the transaction, scoped to this user
	// so another account's token cannot be presented to spare a family.
	var keepFamily *string
	if len(in.KeepTokenHash) > 0 {
		var family string
		err := tx.QueryRow(ctx,
			`SELECT family_id::text FROM app.refresh_tokens
			  WHERE token_hash = $1 AND user_id = $2`,
			in.KeepTokenHash, in.UserID).Scan(&family)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// Unknown token: keep nothing. Deliberately not an error -- the
			// change still has to happen, and the caller is signed out with
			// everyone else.
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

	// `$2::uuid IS NULL OR family_id <> $2` rather than a subquery against the
	// token: a subquery that finds nothing yields NULL, `family_id <> NULL` is
	// NULL, and the UPDATE would then revoke NOTHING. That failure is silent
	// and points the wrong way -- an unknown token must revoke everything.
	const revokeOthers = `
		UPDATE app.refresh_tokens
		   SET revoked_at = $3
		 WHERE user_id = $1
		   AND revoked_at IS NULL
		   AND ($2::uuid IS NULL OR family_id <> $2)`
	if _, err := tx.Exec(ctx, revokeOthers, in.UserID, keepFamily, in.Now); err != nil {
		return fmt.Errorf("revoke other sessions: %w", err)
	}

	if err := insertAudit(ctx, tx, auditEntry{
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
