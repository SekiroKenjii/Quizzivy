package repositories

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/platform/db"
)

// AvatarSweeps is the Postgres store behind domain.AvatarSweeps.
type AvatarSweeps struct {
	db.Repository
}

// NewAvatarSweeps returns the store over dbx.
func NewAvatarSweeps(dbx db.Context) *AvatarSweeps {
	return &AvatarSweeps{Repository: db.NewRepository(dbx)}
}

// Due implements domain.AvatarSweeps.
func (s *AvatarSweeps) Due(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.Query(ctx, `
		SELECT u.id::text
		  FROM app.users u
		 WHERE u.email = 'anonymous-' || u.id::text || '@anonymous.invalid'
		   AND u.disabled_at IS NOT NULL
		   AND u.avatar_key IS NULL
		   AND NOT EXISTS (SELECT 1 FROM app.avatar_sweeps s WHERE s.user_id = u.id)
		 ORDER BY u.id
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("avatar sweeps: due accounts: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("avatar sweeps: due accounts: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("avatar sweeps: due accounts: %w", err)
	}
	return out, nil
}

// Mark implements domain.AvatarSweeps.
func (s *AvatarSweeps) Mark(ctx context.Context, userID string, at time.Time) error {
	if _, err := s.Exec(ctx, `
		INSERT INTO app.avatar_sweeps (user_id, swept_at) VALUES ($1::uuid, $2)
		ON CONFLICT (user_id) DO NOTHING`, userID, at); err != nil {
		return fmt.Errorf("avatar sweeps: mark an account: %w", err)
	}
	return nil
}
