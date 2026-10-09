package repositories

import (
	"context"
	"errors"
	"fmt"
	questionsdomain "quizzivy/internal/modules/questions/domain"
	"quizzivy/internal/modules/tests/domain"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/access"
	"quizzivy/internal/shared/audit"
	"quizzivy/internal/shared/opt"
	"slices"

	"github.com/jackc/pgx/v5"
)

const entityTest = "test"

const liveTests = `t.deleted_at IS NULL`

const scopedTest = `($2::boolean OR owner_id = $3::uuid)`

// QuestionLocks and MediaLocks are the row locks other modules take on this
// module's behalf inside its transactions, so a draft cannot outlive what it
// names, and the checks that run under them. NotOwnedBy reads, without
// locking, which of questions the draft's owner does not own; it runs after
// LockForDraftUse has locked every one of them in the same transaction.
// RequireReadable refuses, as a missing asset, any asset the scope may not
// read; it runs after LockForVersionUse and before any row naming the asset is
// written.
type QuestionLocks interface {
	LockForDraftUse(ctx context.Context, tx pgx.Tx, questionID string) error
	NotOwnedBy(ctx context.Context, tx pgx.Tx, ownerID string, questionIDs []string) ([]string, error)
}

type MediaLocks interface {
	LockForVersionUse(ctx context.Context, tx pgx.Tx, assetID string) error
	RequireReadable(ctx context.Context, tx pgx.Tx, scope access.Scope, assetIDs []string) error
}

type Postgres struct {
	db.Repository
	questions      QuestionLocks
	media          MediaLocks
	groupQuestions GroupQuestionStore
}

func NewPostgres(dbx db.Context, questions QuestionLocks, media MediaLocks) *Postgres {
	return &Postgres{Repository: db.NewRepository(dbx), questions: questions, media: media}
}

// WithGroupQuestions supplies transaction-bound owned-member reads for complete group snapshots.
func (s *Postgres) WithGroupQuestions(questions GroupQuestionStore) *Postgres {
	s.groupQuestions = questions
	return s
}

const testColumns = `
	       t.id::text, t.title, t.description, t.status::text, t.current_version,
	       outline.points::text, outline.questions, outline.audio, outline.skills,
	       uses.live, uses.scheduled, uses.closed,
	       t.created_at, t.updated_at, t.deleted_at`

const testFrom = `
	  FROM app.tests t
	  CROSS JOIN LATERAL (
	    SELECT coalesce(sum(q.points), 0) AS points,
	           count(*) AS questions,
	           count(*) FILTER (WHERE q.media_asset_kind = 'audio' OR EXISTS (
	             SELECT 1 FROM app.group_recordings r WHERE r.group_id=q.context_group_id)) AS audio,
	           coalesce(array_agg(DISTINCT q.skill ORDER BY q.skill) FILTER (WHERE q.skill IS NOT NULL), '{}'::text[]) AS skills
	      FROM (` + draftQuestionRows + `) q
	  ) outline` + testUsage

func scanTest(row pgx.Row) (domain.Test, error) {
	var t domain.Test
	var status string
	err := row.Scan(&t.ID, &t.Title, &t.Description, &status, &t.CurrentVersion,
		&t.TotalPoints, &t.QuestionCount, &t.AudioCount, &t.Skills,
		&t.Assignments.Live, &t.Assignments.Scheduled, &t.Assignments.Closed,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Test{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: scan: %w", err)
	}
	t.Status = domain.Status(status)
	return t, nil
}

// Get returns one live test with its draft outline; a test outside scope is
// domain.ErrNotFound.
func (s *Postgres) Get(ctx context.Context, scope access.Scope, id string) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var locked string
	err = tx.QueryRow(ctx, `SELECT id::text FROM app.tests WHERE id=$1 AND deleted_at IS NULL AND `+scopedTest+` FOR SHARE`,
		id, scope.All, opt.String(scope.UserID)).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Test{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Test{}, err
	}
	result, err := s.get(ctx, tx, id)
	if err != nil {
		return domain.Test{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Test{}, err
	}
	return result, nil
}

func (s *Postgres) get(ctx context.Context, q db.Querier, id string) (domain.Test, error) {
	t, err := scanTest(q.QueryRow(ctx,
		`SELECT`+testColumns+testFrom+` WHERE t.id = $1 AND t.deleted_at IS NULL`, id))
	if err != nil {
		return domain.Test{}, err
	}
	if t.Sections, err = s.loadSections(ctx, q, []string{id}); err != nil {
		return domain.Test{}, err
	}
	return t, nil
}

func (s *Postgres) loadSections(ctx context.Context, q db.Querier, testIDs []string) ([]domain.Section, error) {
	byTest, err := s.sectionsFor(ctx, q, testIDs)
	if err != nil {
		return nil, err
	}
	if len(testIDs) == 1 {
		return byTest[testIDs[0]], nil
	}
	return nil, nil
}

func (s *Postgres) sectionsFor(ctx context.Context, q db.Querier, testIDs []string) (map[string][]domain.Section, error) {
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := readSectionUnits(ctx, q, testIDs, byTest); err != nil {
		return nil, err
	}
	return byTest, nil
}

func (s *Postgres) Create(ctx context.Context, in domain.CreateInput) (domain.Test, error) {
	tx, err := s.Begin(ctx)
	if err != nil {
		return domain.Test{}, fmt.Errorf("tests: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id string
	if err := tx.QueryRow(ctx,
		`INSERT INTO app.tests (title, description, created_by, owner_id) VALUES ($1, $2, $3, coalesce($4::uuid, $3))
		 RETURNING id::text`, in.Title, in.Description, in.ActorID, opt.String(in.OwnerID)).Scan(&id); err != nil {
		return domain.Test{}, fmt.Errorf("tests: insert: %w", err)
	}
	if err := audit.Write(ctx, tx, audit.Entry{
		ActorUserID: &in.ActorID,
		Action:      "test.created",
		Entity:      entityTest,
		EntityID:    &id,
		OccurredAt:  in.Now,
		IP:          opt.String(in.IP),
		UserAgent:   opt.String(in.UserAgent),
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

func (s *Postgres) lockQuestions(ctx context.Context, tx pgx.Tx, ownerID string, sections []domain.SectionInput) error {
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
	if len(ids) == 0 {
		return nil
	}
	foreign, err := s.questions.NotOwnedBy(ctx, tx, ownerID, ids)
	if err != nil {
		return err
	}
	if len(foreign) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrUnknownQuestion, foreign[0])
	}
	return nil
}
