package repositories

import (
	"context"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
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

	source, err := s.get(ctx, tx, in.ID)
	if err != nil {
		return domain.Test{}, err
	}

	var copyID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by)
		 SELECT title, description, $2 FROM app.tests WHERE id = $1
		 RETURNING id::text`, in.ID, in.ActorID).Scan(&copyID); err != nil {
		return domain.Test{}, fmt.Errorf("tests: copy test row: %w", err)
	}

	for _, sec := range source.Sections {
		var sectionID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.test_sections (test_id, ordinal, title, instructions)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			copyID, sec.Ordinal, sec.Title, sec.Instructions).Scan(&sectionID); err != nil {
			return domain.Test{}, fmt.Errorf("tests: copy section: %w", err)
		}
		if err := writeSectionQuestions(ctx, tx, sectionID, sec.QuestionIDs); err != nil {
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
