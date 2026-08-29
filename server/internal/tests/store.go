package tests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"quizzivy/internal/audit"
	"quizzivy/internal/questions"
)

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// testColumns is explicit rather than SELECT *.
//
// total_points and question_count are derived from the draft outline in SQL, so
// the list does not need a second pass per row to compute them.
const testColumns = `
	       t.id::text, t.title, t.description, t.status::text, t.current_version,
	       coalesce((SELECT sum(q.points)
	                   FROM app.test_sections s
	                   JOIN app.test_section_questions sq ON sq.test_section_id = s.id
	                   JOIN app.questions q ON q.id = sq.question_id
	                  WHERE s.test_id = t.id), 0)::text,
	       (SELECT count(*)
	          FROM app.test_sections s
	          JOIN app.test_section_questions sq ON sq.test_section_id = s.id
	         WHERE s.test_id = t.id),
	       (SELECT count(*)
	          FROM app.test_sections s
	          JOIN app.test_section_questions sq ON sq.test_section_id = s.id
	          JOIN app.questions q ON q.id = sq.question_id
	         WHERE s.test_id = t.id AND q.media_asset_kind = 'audio'),
	       t.created_at, t.updated_at, t.deleted_at`

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func scanTest(row pgx.Row) (Test, error) {
	var t Test
	var status string
	err := row.Scan(&t.ID, &t.Title, &t.Description, &status, &t.CurrentVersion,
		&t.TotalPoints, &t.QuestionCount, &t.AudioCount, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Test{}, ErrNotFound
	}
	if err != nil {
		return Test{}, fmt.Errorf("tests: scan: %w", err)
	}
	t.Status = Status(status)
	return t, nil
}

// Get returns one live test with its draft outline.
func (s *Store) Get(ctx context.Context, id string) (Test, error) {
	return s.get(ctx, s.pool, id)
}

func (s *Store) get(ctx context.Context, q querier, id string) (Test, error) {
	t, err := scanTest(q.QueryRow(ctx,
		`SELECT`+testColumns+` FROM app.tests t WHERE t.id = $1 AND t.deleted_at IS NULL`, id))
	if err != nil {
		return Test{}, err
	}
	if t.Sections, err = s.loadSections(ctx, q, []string{id}); err != nil {
		return Test{}, err
	}
	return t, nil
}

// loadSections reads the outline for a whole page in one query per level, so a
// list does not cost two round trips per test.
func (s *Store) loadSections(ctx context.Context, q querier, testIDs []string) ([]Section, error) {
	byTest, err := s.sectionsFor(ctx, q, testIDs)
	if err != nil {
		return nil, err
	}
	if len(testIDs) == 1 {
		return byTest[testIDs[0]], nil
	}
	return nil, nil
}

func (s *Store) sectionsFor(ctx context.Context, q querier, testIDs []string) (map[string][]Section, error) {
	byTest := make(map[string][]Section, len(testIDs))
	if len(testIDs) == 0 {
		return byTest, nil
	}

	rows, err := q.Query(ctx,
		`SELECT s.test_id::text, s.id::text, s.ordinal, s.title, s.instructions,
		        coalesce(array_agg(sq.question_id::text ORDER BY sq.ordinal)
		                 FILTER (WHERE sq.question_id IS NOT NULL), '{}')
		   FROM app.test_sections s
		   LEFT JOIN app.test_section_questions sq ON sq.test_section_id = s.id
		  WHERE s.test_id = ANY($1::uuid[])
		  GROUP BY s.test_id, s.id, s.ordinal, s.title, s.instructions
		  ORDER BY s.test_id, s.ordinal`, testIDs)
	if err != nil {
		return nil, fmt.Errorf("tests: load sections: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var testID string
		var sec Section
		if err := rows.Scan(&testID, &sec.ID, &sec.Ordinal, &sec.Title, &sec.Instructions,
			&sec.QuestionIDs); err != nil {
			return nil, fmt.Errorf("tests: scan section: %w", err)
		}
		byTest[testID] = append(byTest[testID], sec)
	}
	return byTest, rows.Err()
}

// CreateInput is a new empty draft.
type CreateInput struct {
	Title       string
	Description *string
	ActorID     string
	Now         time.Time
	IP          string
	UserAgent   string
}

func (s *Store) Create(ctx context.Context, in CreateInput) (Test, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Test{}, fmt.Errorf("tests: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by) VALUES ($1, $2, $3)
		 RETURNING id::text`, in.Title, in.Description, in.ActorID).Scan(&id); err != nil {
		return Test{}, fmt.Errorf("tests: insert: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.created",
		Entity:      "test",
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return Test{}, err
	}

	created, err := s.get(ctx, tx, id)
	if err != nil {
		return Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Test{}, fmt.Errorf("tests: commit create: %w", err)
	}
	return created, nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// lockQuestions takes the row lock on every question an outline names, so a
// concurrent soft delete of one of them cannot slip between the check and the
// write (see questions.LockForDraftUse).
//
// Sorted, because two outline writes naming overlapping questions in different
// orders would otherwise deadlock.
func lockQuestions(ctx context.Context, tx pgx.Tx, sections []SectionInput) error {
	seen := map[string]bool{}
	var ids []string
	for _, sec := range sections {
		for _, id := range sec.QuestionIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)

	for _, id := range ids {
		if err := questions.LockForDraftUse(ctx, tx, id); err != nil {
			if errors.Is(err, questions.ErrNotFound) {
				return fmt.Errorf("%w: %s", ErrUnknownQuestion, id)
			}
			return err
		}
	}
	return nil
}
