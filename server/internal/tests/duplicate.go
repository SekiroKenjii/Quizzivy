package tests

import (
	"context"
	"fmt"
	"time"

	"quizzivy/internal/audit"
)

// DuplicateInput copies a test's draft structure.
type DuplicateInput struct {
	ID        string
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}

// Duplicate copies the draft outline and nothing else.
//
// The copy starts as a draft with current_version 0: versions are snapshots of
// a publish that happened, and the copy has not been published. Copying them
// would give the new test a history it never had, and an assignment pointing at
// a version whose test never went through publish validation.
func (s *Store) Duplicate(ctx context.Context, in DuplicateInput) (Test, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Test{}, fmt.Errorf("tests: begin duplicate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	source, err := s.get(ctx, tx, in.ID)
	if err != nil {
		return Test{}, err
	}

	var copyID string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by)
		 SELECT title, description, $2 FROM app.tests WHERE id = $1
		 RETURNING id::text`, in.ID, in.ActorID).Scan(&copyID); err != nil {
		return Test{}, fmt.Errorf("tests: copy test row: %w", err)
	}

	for _, sec := range source.Sections {
		var sectionID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.test_sections (test_id, ordinal, title, instructions)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			copyID, sec.Ordinal, sec.Title, sec.Instructions).Scan(&sectionID); err != nil {
			return Test{}, fmt.Errorf("tests: copy section: %w", err)
		}
		if err := writeSectionQuestions(ctx, tx, sectionID, sec.QuestionIDs); err != nil {
			return Test{}, err
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.duplicated",
		Entity:      entityTest,
		EntityID:    &copyID,
		OccurredAt:  in.Now,
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return Test{}, err
	}

	created, err := s.get(ctx, tx, copyID)
	if err != nil {
		return Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Test{}, fmt.Errorf("tests: commit duplicate: %w", err)
	}
	return created, nil
}
