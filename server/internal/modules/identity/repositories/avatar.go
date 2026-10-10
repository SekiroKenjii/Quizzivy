package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
)

// SetAvatar stores the caller's photo key, or clears it for a nil key, and
// writes one audit row when the key changed, in one statement. It answers the
// caller as stored and the key the photo had before, which only the statement
// can know without a race: two concurrent writes each see the other's key as
// the previous one, so each deletes the object the other replaced.
func (s *Users) SetAvatar(ctx context.Context, in domain.AvatarRecord) (domain.AvatarWrite, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.AvatarWrite{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const statement = `
 WITH changed AS (
  UPDATE app.users SET avatar_key = $2::text
   WHERE id = $1::uuid AND disabled_at IS NULL
  RETURNING OLD.avatar_key AS old_key, NEW.avatar_key AS new_key
 ), audited AS (
  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
  SELECT $1::uuid, CASE WHEN new_key IS NULL THEN 'user.avatar_removed' ELSE 'user.avatar_set' END,
         'user', $1::uuid, $3, $4::inet, $5, jsonb_build_object('field', 'avatar')
    FROM changed
   WHERE old_key IS DISTINCT FROM new_key
 ) SELECT (SELECT old_key FROM changed), EXISTS (SELECT 1 FROM changed)`
	var previous *string
	var updated bool
	if err := tx.QueryRow(ctx, statement, in.UserID, in.Key, in.Now, in.IP, in.UserAgent).Scan(&previous, &updated); err != nil {
		return domain.AvatarWrite{}, fmt.Errorf("set avatar: %w", err)
	}
	user, err := persistedUser(ctx, tx, in.UserID, updated)
	if err != nil {
		return domain.AvatarWrite{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AvatarWrite{}, err
	}
	return domain.AvatarWrite{User: user, PreviousKey: previous}, nil
}
