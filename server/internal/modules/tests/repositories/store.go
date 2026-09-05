package repositories

import (
	"context"
	"errors"
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/shared/audit"
	"slices"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const entityTest = "test"

const liveTests = `t.deleted_at IS NULL`

// QuestionLocks and MediaLocks are the row locks other modules take on this
// module's behalf inside its transactions, so a draft cannot outlive what it names.
type QuestionLocks interface {
	LockForDraftUse(ctx context.Context, tx pgx.Tx, questionID string) error
}

type MediaLocks interface {
	LockForVersionUse(ctx context.Context, tx pgx.Tx, assetID string) error
}

type Postgres struct {
	pool      *pgxpool.Pool
	questions QuestionLocks
	media     MediaLocks
}

func NewPostgres(pool *pgxpool.Pool, questions QuestionLocks, media MediaLocks) *Postgres {
	return &Postgres{pool: pool, questions: questions, media: media}
}

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

func scanTest(row pgx.Row) (domain.Test, error) {
	var t domain.Test
	var status string
	err := row.Scan(&t.ID, &t.Title, &t.Description, &status, &t.CurrentVersion,
		&t.TotalPoints, &t.QuestionCount, &t.AudioCount, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Test{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: scan: %w", err)
	}
	t.Status = domain.Status(status)
	return t, nil
}

// Get returns one live test with its draft outline.
func (s *Postgres) Get(ctx context.Context, id string) (domain.Test, error) {
	return s.get(ctx, s.pool, id)
}

func (s *Postgres) get(ctx context.Context, q querier, id string) (domain.Test, error) {
	t, err := scanTest(q.QueryRow(ctx,
		`SELECT`+testColumns+` FROM app.tests t WHERE t.id = $1 AND t.deleted_at IS NULL`, id))
	if err != nil {
		return domain.Test{}, err
	}
	if t.Sections, err = s.loadSections(ctx, q, []string{id}); err != nil {
		return domain.Test{}, err
	}
	return t, nil
}

func (s *Postgres) loadSections(ctx context.Context, q querier, testIDs []string) ([]domain.Section, error) {
	byTest, err := s.sectionsFor(ctx, q, testIDs)
	if err != nil {
		return nil, err
	}
	if len(testIDs) == 1 {
		return byTest[testIDs[0]], nil
	}
	return nil, nil
}

func (s *Postgres) sectionsFor(ctx context.Context, q querier, testIDs []string) (map[string][]domain.Section, error) {
	byTest := make(map[string][]domain.Section, len(testIDs))
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
		var sec domain.Section
		if err := rows.Scan(&testID, &sec.ID, &sec.Ordinal, &sec.Title, &sec.Instructions,
			&sec.QuestionIDs); err != nil {
			return nil, fmt.Errorf("tests: scan section: %w", err)
		}
		byTest[testID] = append(byTest[testID], sec)
	}
	return byTest, rows.Err()
}

func (s *Postgres) Create(ctx context.Context, in domain.CreateInput) (domain.Test, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by) VALUES ($1, $2, $3)
		 RETURNING id::text`, in.Title, in.Description, in.ActorID).Scan(&id); err != nil {
		return domain.Test{}, fmt.Errorf("tests: insert: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.created",
		Entity:      entityTest,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          optional(in.IP),
		UserAgent:   optional(in.UserAgent),
	}); err != nil {
		return domain.Test{}, err
	}

	created, err := s.get(ctx, tx, id)
	if err != nil {
		return domain.Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Test{}, fmt.Errorf("tests: commit create: %w", err)
	}
	return created, nil
}

func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (s *Postgres) lockQuestions(ctx context.Context, tx pgx.Tx, sections []domain.SectionInput) error {
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
		if err := s.questions.LockForDraftUse(ctx, tx, id); err != nil {
			if errors.Is(err, questionsdomain.ErrNotFound) {
				return fmt.Errorf("%w: %s", domain.ErrUnknownQuestion, id)
			}
			return err
		}
	}
	return nil
}
