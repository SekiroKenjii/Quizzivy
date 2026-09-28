package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/classes/domain"

	"github.com/jackc/pgx/v5"
)

// LookupByCode finds the newest code row stored under any of the code's
// candidate hashes, revoked or not, with its class's own teacher. The caller
// still checks the row against the candidate of its own scheme and key.
func (s *Postgres) LookupByCode(ctx context.Context, code domain.JoinCodeLookup) (*domain.CodeRow, error) {
	const q = `
		SELECT c.id::text, c.name, c.self_join_enabled AND c.archived_at IS NULL,
		       jc.lookup_scheme, jc.key_id, jc.code_hash,
		       jc.revoked_at, jc.expires_at, jc.max_uses, jc.uses_count,
		       (SELECT t.full_name FROM app.users t WHERE t.id = c.teacher_id)
		  FROM app.class_join_codes jc
		  JOIN app.classes c ON c.id = jc.class_id
		 WHERE jc.code_hash = ANY($1::bytea[])
		 ORDER BY jc.created_at DESC, jc.id DESC
		 LIMIT 1`

	var r domain.CodeRow
	err := s.QueryRow(ctx, q, code.Hashes()).Scan(
		&r.ClassID, &r.ClassName, &r.SelfJoinEnabled,
		&r.Lookup.Scheme, &r.Lookup.KeyID, &r.Lookup.Hash,
		&r.RevokedAt, &r.ExpiresAt, &r.MaxUses, &r.UsesCount,
		&r.TeacherName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("look up join code: %w", err)
	}
	return &r, nil
}
