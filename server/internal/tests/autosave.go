package tests

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"quizzivy/internal/audit"
)

// UpdateRequest is one autosave.
type UpdateRequest struct {
	ID        string
	Input     UpdateInput
	ActorID   string
	Now       time.Time
	IP        string
	UserAgent string
}

// Update applies an autosave: metadata and, when present, the whole outline, in
// one transaction guarded on the version the client read.
func (s *Store) Update(ctx context.Context, in UpdateRequest) (Test, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Test{}, fmt.Errorf("tests: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := checkVersion(ctx, tx, in.ID, in.Input.ExpectedUpdatedAt); err != nil {
		return Test{}, err
	}

	// Locked before anything is read from app.questions, and before the tests
	// row is touched, so the ordering is the same for every writer.
	if in.Input.SetSections {
		if err := lockQuestions(ctx, tx, in.Input.Sections); err != nil {
			return Test{}, err
		}
	}

	if err := applyMetadata(ctx, tx, in); err != nil {
		return Test{}, err
	}
	if in.Input.SetSections {
		if err := replaceOutline(ctx, tx, in.ID, in.Input.Sections); err != nil {
			return Test{}, err
		}
	}

	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.updated",
		Entity:      "test",
		EntityID:    &in.ID,
		OccurredAt:  in.Now,
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return Test{}, err
	}

	saved, err := s.get(ctx, tx, in.ID)
	if err != nil {
		return Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Test{}, fmt.Errorf("tests: commit update: %w", err)
	}
	return saved, nil
}

// checkVersion locks the test and compares the client's version.
//
// Compared at microsecond precision, which is what timestamptz stores: the
// value came from this column and round-tripped through JSON, so anything
// finer would be a difference the client could not have introduced.
func checkVersion(ctx context.Context, tx pgx.Tx, id string, expected time.Time) error {
	var current time.Time
	err := tx.QueryRow(ctx,
		`SELECT updated_at FROM app.tests WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
		id).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("tests: lock for update: %w", err)
	}
	if !current.Truncate(time.Microsecond).Equal(expected.Truncate(time.Microsecond)) {
		return ErrStaleWrite
	}
	return nil
}

// applyMetadata always writes the tests row, even when no field changed, so the
// updated_at trigger advances the version. Without that, two successive
// outline-only saves would both pass the same guard.
func applyMetadata(ctx context.Context, tx pgx.Tx, in UpdateRequest) error {
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

func statusArg(s *Status) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

// replaceOutline writes the sections and their question ordering.
//
// Sections are diffed by id rather than deleted and recreated, so an id the
// builder is holding stays valid across a save. The questions within a section
// are position-addressed, so those rows are simply rewritten.
//
// Ordinals are deferred (D-13): they are rewritten one row at a time and
// transiently collide whenever two sections swap places.
func replaceOutline(ctx context.Context, tx pgx.Tx, testID string, sections []SectionInput) error {
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

func upsertSection(ctx context.Context, tx pgx.Tx, testID string, ordinal int, sec SectionInput) (string, error) {
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
		// A section id from another test, or one already removed. Reported as
		// not-found rather than silently creating a section the client will not
		// recognise.
		return "", ErrNotFound
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
