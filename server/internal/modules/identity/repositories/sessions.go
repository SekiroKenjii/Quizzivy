package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/shared/audit"
	"time"

	"github.com/jackc/pgx/v5"
)

// ListSessions returns the user's live sessions, the one the presented cookie
// names first, then the most recently used, at most q.Limit of them. A
// session is a refresh-token family with an unrevoked, unexpired token; a
// rotation consumes the predecessor in the transaction that issues its
// successor, so the family has one such token and its row is the latest
// sign-in or refresh. A cookie of another user, or one that names no live
// family, makes no session current.
func (s *Users) ListSessions(ctx context.Context, q domain.SessionsQuery) ([]domain.Session, error) {
	const list = `
		WITH mine AS (
		  SELECT family_id
		    FROM app.refresh_tokens
		   WHERE token_hash = $2 AND user_id = $1::uuid
		), live AS (
		  SELECT DISTINCT ON (family_id) family_id, issued_at, user_agent, geo_label
		    FROM app.refresh_tokens
		   WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > $3
		   ORDER BY family_id, issued_at DESC, id DESC
		)
		SELECT l.family_id::text, l.user_agent, l.geo_label, l.issued_at,
		       l.family_id IN (SELECT family_id FROM mine) AS is_current
		  FROM live l
		 ORDER BY is_current DESC, l.issued_at DESC, l.family_id
		 LIMIT $4`
	rows, err := s.Query(ctx, list, q.UserID, q.CurrentTokenHash, q.Now, q.Limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	sessions := []domain.Session{}
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(&session.FamilyID, &session.UserAgent, &session.GeoLabel, &session.LastUsedAt, &session.Current); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	return sessions, nil
}

// RevokeSession ends one live session of the user, moves the user's session
// epoch and audits it, in one transaction that first takes the lock a rotation
// takes, so a rotation of that family either commits first and its successor
// is revoked here, or waits and finds the family revoked. It answers
// ErrNoCurrentSession when the cookie names no live session of the user, then
// ErrSessionNotFound when the user has no live session with that id, then
// ErrSessionIsCurrent when it is the cookie's own, writing nothing in each
// case. The caller forgets the cached principal after it returns nil.
func (s *Users) RevokeSession(ctx context.Context, in domain.RevokeSessionRecord) error {
	tx, err := s.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin session revocation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUser(ctx, tx, in.UserID); err != nil {
		return err
	}
	current, err := currentFamily(ctx, tx, in.UserID, in.CurrentTokenHash, in.Now)
	if err != nil {
		return err
	}
	live, err := liveFamily(ctx, tx, in.UserID, in.FamilyID, in.Now)
	if err != nil {
		return err
	}
	switch {
	case !live:
		return domain.ErrSessionNotFound
	case in.FamilyID == current:
		return domain.ErrSessionIsCurrent
	}

	if _, err := tx.Exec(ctx, `
		UPDATE app.refresh_tokens
		   SET revoked_at = $3
		 WHERE family_id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`,
		in.FamilyID, in.UserID, in.Now); err != nil {
		return fmt.Errorf("revoke session %s: %w", in.FamilyID, err)
	}
	if err := bumpSessionEpoch(ctx, tx, in.UserID); err != nil {
		return err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.UserID,
		Action:      "session.revoked",
		Entity:      "refresh_token_family",
		EntityID:    &in.FamilyID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit session revocation: %w", err)
	}
	return nil
}

// RevokeOtherSessions ends every live session of the user except the one the
// cookie names, and returns how many it ended. Only when it ended one does it
// move the user's session epoch and audit, in the same transaction; with none
// to end it writes nothing. The caller forgets the cached principal after a
// non-zero count. It answers ErrNoCurrentSession when the cookie names no live
// session of the user.
func (s *Users) RevokeOtherSessions(ctx context.Context, in domain.RevokeOtherSessionsRecord) (int, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin session revocation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockUser(ctx, tx, in.UserID); err != nil {
		return 0, err
	}
	current, err := currentFamily(ctx, tx, in.UserID, in.CurrentTokenHash, in.Now)
	if err != nil {
		return 0, err
	}

	var revoked int
	if err := tx.QueryRow(ctx, `
		WITH ended AS (
		  UPDATE app.refresh_tokens
		     SET revoked_at = $3
		   WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > $3 AND family_id <> $2::uuid
		  RETURNING family_id
		)
		SELECT count(DISTINCT family_id) FROM ended`,
		in.UserID, current, in.Now).Scan(&revoked); err != nil {
		return 0, fmt.Errorf("revoke other sessions: %w", err)
	}
	if revoked == 0 {
		return 0, nil
	}
	if err := bumpSessionEpoch(ctx, tx, in.UserID); err != nil {
		return 0, err
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.UserID,
		Action:      "session.revoked_others",
		Entity:      "user",
		EntityID:    &in.UserID,
		OccurredAt:  in.Now,
		IP:          in.IP,
		UserAgent:   in.UserAgent,
		Diff:        []byte(fmt.Sprintf(`{"revoked":%d}`, revoked)),
	}); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit session revocation: %w", err)
	}
	return revoked, nil
}

func lockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var locked bool
	err := tx.QueryRow(ctx,
		`SELECT true FROM app.users WHERE id = $1::uuid FOR NO KEY UPDATE`, userID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return fmt.Errorf("lock user %s: %w", userID, err)
	}
	return nil
}

func currentFamily(ctx context.Context, tx pgx.Tx, userID string, tokenHash []byte, now time.Time) (string, error) {
	if len(tokenHash) == 0 {
		return "", domain.ErrNoCurrentSession
	}
	var family string
	err := tx.QueryRow(ctx, `
		SELECT c.family_id::text
		  FROM app.refresh_tokens c
		 WHERE c.token_hash = $1 AND c.user_id = $2::uuid
		   AND EXISTS (SELECT 1
		                 FROM app.refresh_tokens l
		                WHERE l.family_id = c.family_id AND l.user_id = $2::uuid
		                  AND l.revoked_at IS NULL AND l.expires_at > $3)`,
		tokenHash, userID, now).Scan(&family)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNoCurrentSession
	}
	if err != nil {
		return "", fmt.Errorf("resolve current session: %w", err)
	}
	return family, nil
}

func liveFamily(ctx context.Context, tx pgx.Tx, userID, familyID string, now time.Time) (bool, error) {
	var live bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1
		                 FROM app.refresh_tokens
		                WHERE family_id = $1::uuid AND user_id = $2::uuid
		                  AND revoked_at IS NULL AND expires_at > $3)`,
		familyID, userID, now).Scan(&live); err != nil {
		return false, fmt.Errorf("look up session %s: %w", familyID, err)
	}
	return live, nil
}

func bumpSessionEpoch(ctx context.Context, tx pgx.Tx, userID string) error {
	if _, err := tx.Exec(ctx,
		`UPDATE app.users SET session_epoch = session_epoch + 1 WHERE id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("move session epoch of %s: %w", userID, err)
	}
	return nil
}
