package repositories

import (
	"context"
	"errors"
	"fmt"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Update applies an autosave: metadata and, when present, the whole outline, in
// one transaction guarded on the version the client read.
func (s *Postgres) Update(ctx context.Context, in domain.UpdateRequest) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := checkVersion(ctx, tx, in.ID, in.Input.ExpectedUpdatedAt); err != nil {
		return domain.Test{}, err
	}

	if in.Input.SetSections {
		if err := s.lockQuestions(ctx, tx, in.Input.Sections); err != nil {
			return domain.Test{}, err
		}
	}

	if err := applyMetadata(ctx, tx, in); err != nil {
		return domain.Test{}, err
	}
	if in.Input.SetSections {
		if err := replaceOutline(ctx, tx, in.ID, in.Input.Sections); err != nil {
			return domain.Test{}, err
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.updated",
		Entity:      entityTest,
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
	}); err != nil {
		return domain.Test{}, err
	}

	saved, err := s.get(ctx, tx, in.ID)
	if err != nil {
		return domain.Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Test{}, fmt.Errorf("tests: commit update: %w", err)
	}
	return saved, nil
}

func checkVersion(ctx context.Context, tx pgx.Tx, id string, expected time.Time) error {
	var current time.Time
	err := tx.QueryRow(ctx,
		`SELECT updated_at FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("tests: lock for update: %w", err)
	}
	if !current.Truncate(time.Microsecond).Equal(expected.Truncate(time.Microsecond)) {
		return domain.ErrStaleWrite
	}
	return nil
}

func applyMetadata(ctx context.Context, tx pgx.Tx, in domain.UpdateRequest) error {
	_, err := tx.Exec(ctx, `
		UPDATE app.tests
		   SET title = coalesce($2, title),
		       description = CASE WHEN $3::boolean THEN $4 ELSE description END,
		       status = coalesce($5::app.test_status, status)
		 WHERE id = $1`,
		in.ID, in.Input.Title, in.Input.SetDescription, in.Input.Description, statusArg(in.Input.Status))
	if err != nil {
		return fmt.Errorf("tests: update metadata: %w", err)
	}
	return nil
}

func statusArg(s *domain.Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func replaceOutline(ctx context.Context, tx pgx.Tx, testID string, sections []domain.SectionInput) error {
	if _, err := tx.Exec(ctx,
		`SET CONSTRAINTS app.test_sections_ordinal_key,
		                 app.test_section_questions_ordinal_key DEFERRED`); err != nil {
		return fmt.Errorf("tests: defer ordinal constraints: %w", err)
	}

	keep := make([]string, 0, len(sections))
	for _, sec := range sections {
		if sec.ID != "" {
			keep = append(keep, sec.ID)
		}
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.test_sections WHERE test_id = $1 AND NOT (id = ANY($2::uuid[]))`,
		testID, keep); err != nil {
		return fmt.Errorf("tests: drop removed sections: %w", err)
	}

	for ordinal, sec := range sections {
		id, err := upsertSection(ctx, tx, testID, ordinal, sec)
		if err != nil {
			return err
		}
		if err := writeSectionQuestions(ctx, tx, id, sec.QuestionIDs); err != nil {
			return err
		}
	}
	return nil
}

func upsertSection(ctx context.Context, tx pgx.Tx, testID string, ordinal int, sec domain.SectionInput) (string, error) {
	if sec.ID == "" {
		var id string
		if err := tx.QueryRow(ctx,
			`INSERT INTO app.test_sections (test_id, ordinal, title, instructions)
			 VALUES ($1, $2, $3, $4) RETURNING id::text`,
			testID, ordinal, sec.Title, sec.Instructions).Scan(&id); err != nil {
			return "", fmt.Errorf("tests: insert section: %w", err)
		}
		return id, nil
	}

	tag, err := tx.Exec(ctx,
		`UPDATE app.test_sections SET ordinal = $3, title = $4, instructions = $5
		  WHERE id = $1 AND test_id = $2`,
		sec.ID, testID, ordinal, sec.Title, sec.Instructions)
	if err != nil {
		return "", fmt.Errorf("tests: update section: %w", err)
	}
	if tag.RowsAffected() == 0 {

		return "", domain.ErrNotFound
	}
	return sec.ID, nil
}

func writeSectionQuestions(ctx context.Context, tx pgx.Tx, sectionID string, questionIDs []string) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM app.test_section_questions WHERE test_section_id = $1`, sectionID); err != nil {
		return fmt.Errorf("tests: clear section questions: %w", err)
	}
	if len(questionIDs) == 0 {
		return nil
	}

	rows := make([][]any, len(questionIDs))
	for i, questionID := range questionIDs {
		rows[i] = []any{sectionID, i, questionID}
	}
	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"app", "test_section_questions"},
		[]string{"test_section_id", "ordinal", "question_id"},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("tests: write section questions: %w", err)
	}
	return nil
}
