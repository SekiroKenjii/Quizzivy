package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"

	"github.com/jackc/pgx/v5"
)

// LookupByCodeHash finds a code by its hash, revoked or not.
func (s *Postgres) LookupByCodeHash(ctx context.Context, hash []byte) (*domain.CodeRow, error) {
	const q = `
		SELECT c.id::text, c.name, c.self_join_enabled AND c.archived_at IS NULL,
		       jc.code_hash, jc.revoked_at, jc.expires_at, jc.max_uses, jc.uses_count,
		       (SELECT u.full_name
		          FROM app.users u
		         WHERE u.role = 'admin' AND u.disabled_at IS NULL
		         ORDER BY u.created_at
		         LIMIT 1)
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.code_hash = $1`

	var r domain.CodeRow
	err := s.QueryRow(ctx, q, hash).Scan(
		&r.ClassID, &r.ClassName, &r.SelfJoinEnabled,
		&r.CodeHash, &r.RevokedAt, &r.ExpiresAt, &r.MaxUses, &r.UsesCount,
		&r.TeacherName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up join code: %w", err)
	}
	return &r, nil
}
