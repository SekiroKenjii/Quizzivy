package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
)

// Duplicate copies the draft outline and nothing else.
func (s *Postgres) Duplicate(ctx context.Context, in domain.DuplicateInput) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: begin duplicate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	owner, err := lockDuplicateSource(ctx, tx, in.ID, in.Scope)
	if err != nil {
		return domain.Test{}, err
	}
	source, err := s.get(ctx, tx, in.ID)
	if err != nil {
		return domain.Test{}, err
	}

	var copyID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by, owner_id)
		 SELECT title, description, $2, owner_id FROM app.tests WHERE id = $1
		 RETURNING id::text`, in.ID, in.ActorID).Scan(&copyID); err != nil {
		return domain.Test{}, fmt.Errorf("tests: copy test row: %w", err)
	}

	if len(source.Sections) > 0 {
		if err := lockDraftContent(ctx, tx, in.ID, true); err != nil {
			return domain.Test{}, err
		}
		draft, err := s.loadDraftSnapshot(ctx, tx, in.ID)
		if err != nil {
			return domain.Test{}, err
		}
		if err := s.copyDraftGraph(ctx, tx, copyID, owner, draft, in); err != nil {
			return domain.Test{}, err
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.duplicated",
		Entity:      entityTest,
		EntityID:    &copyID,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
	}); err != nil {
		return domain.Test{}, err
	}

	created, err := s.get(ctx, tx, copyID)
	if err != nil {
		return domain.Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Test{}, fmt.Errorf("tests: commit duplicate: %w", err)
	}
	return created, nil
}

func lockDuplicateSource(ctx context.Context, tx pgx.Tx, id string, scope access.Scope) (string, error) {
	var owner string
	err := tx.QueryRow(ctx, `SELECT owner_id::text FROM app.tests WHERE id=$1 AND deleted_at IS NULL AND `+scopedTest+` FOR UPDATE`,
		id, scope.All, opt.String(scope.UserID)).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	return owner, err
}
