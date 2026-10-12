package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/identity/domain"
	"quizzivy/internal/platform/db"
)

// UpdatePreferences atomically merges the current row and audits actual changed top-level keys.
func (s *Users) UpdatePreferences(ctx context.Context, in domain.PreferencesRecord) (domain.Preferences, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Preferences{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const statement = `
 WITH changed AS (
  UPDATE app.users SET preferences = preferences || $2::jsonb
  WHERE id = $1::uuid AND disabled_at IS NULL
  RETURNING OLD.preferences AS before, NEW.preferences AS after
 ), audited AS (
  INSERT INTO app.audit_log (actor_user_id, action, entity, entity_id, occurred_at, ip, user_agent, diff)
  SELECT $1::uuid, 'user.preferences_updated', 'user', $1::uuid, $3, $4::inet, $5,
   jsonb_build_object('fields', fields)
  FROM changed CROSS JOIN LATERAL (
   SELECT array_agg(key ORDER BY key) AS fields
   FROM jsonb_object_keys(after) AS key WHERE before->key IS DISTINCT FROM after->key
  ) AS differences WHERE cardinality(fields) > 0
 ) SELECT EXISTS (SELECT 1 FROM changed)`
	var updated bool
	if err := tx.QueryRow(ctx, statement, in.UserID, in.Patch, in.Now, in.IP, in.UserAgent).Scan(&updated); err != nil {
		if db.IsCheckViolation(err, "users_preferences_bytes_check") {
			return domain.Preferences{}, domain.ErrPreferencesTooLarge
		}
		if db.IsCheckViolation(err, "users_preferences_object_check") {
			return domain.Preferences{}, domain.ErrPreferencesInvalid
		}
		return domain.Preferences{}, fmt.Errorf("update preferences: %w", err)
	}
	user, err := persistedUser(ctx, tx, in.UserID, updated)
	if err != nil {
		return domain.Preferences{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Preferences{}, err
	}
	return user.Preferences, nil
}
