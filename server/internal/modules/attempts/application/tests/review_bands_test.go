//go:build integration

package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// essayPaper is seedPaper with both questions made essays: rubric (3 points)
// has four criteria and plain (6 points) none.
type essayPaper struct {
	paper
	rubric, plain string
	criteria      []string
}

func seedEssayPaper(t *testing.T, pool *pgxpool.Pool) essayPaper {
	t.Helper()
	ctx := context.Background()
	p := essayPaper{paper: seedPaper(t, pool, "submitted")}
	p.rubric, p.plain = p.choice, p.essay
	for _, q := range []string{
		`DELETE FROM app.test_version_options WHERE test_version_question_id = $1::uuid`,
		`UPDATE app.test_version_questions SET type = 'essay', points = '3.00' WHERE id = $1::uuid`,
		`UPDATE app.attempt_answers SET requires_manual = true, auto_score = NULL, payload = '{"type":"text","value":"An essay."}' WHERE question_id = $1::uuid`,
	} {
		if _, err := pool.Exec(ctx, q, p.rubric); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET type = 'essay', points = '6.00' WHERE id = $1::uuid`, p.plain); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO app.test_version_rubric_criteria (test_version_question_id, ordinal, label)
			VALUES ($1::uuid, $2, 'Tiêu chí') RETURNING id::text`, p.rubric, i).Scan(&id); err != nil {
			t.Fatal(err)
		}
		p.criteria = append(p.criteria, id)
	}
	return p
}

func (p essayPaper) bands(bands ...float64) []domain.CriterionBand {
	out := make([]domain.CriterionBand, len(bands))
	for i, b := range bands {
		out[i] = domain.CriterionBand{CriterionID: p.criteria[i], Band: b}
	}
	return out
}

type storedMark struct {
	manual, band *float64
	criteria     []float64
}

func markOf(t *testing.T, pool *pgxpool.Pool, attempt, question string) storedMark {
	t.Helper()
	var m storedMark
	if err := pool.QueryRow(context.Background(), `
		SELECT a.manual_score, a.band,
		       coalesce((SELECT array_agg(c.band ORDER BY r.ordinal)
		                   FROM app.attempt_answer_criteria c
		                   JOIN app.test_version_rubric_criteria r ON r.id = c.criterion_id
		                  WHERE c.attempt_id = a.attempt_id AND c.question_id = a.question_id), '{}')
		  FROM app.attempt_answers a WHERE a.attempt_id = $1::uuid AND a.question_id = $2::uuid`,
		attempt, question).Scan(&m.manual, &m.band, &m.criteria); err != nil {
		t.Fatal(err)
	}
	return m
}

func is(got *float64, want float64) bool { return got != nil && *got == want }

func TestAnEssayIsGradedOnItsBandOrItsCriteriaAndThePaperOnTheirWeightedMean(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	grader := domain.Request{ActorID: p.admin}

	score, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.plain, Band: pts(7)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.plain); !is(got.manual, 4.67) || !is(got.band, 7) || len(got.criteria) != 0 {
		t.Errorf("band 7 of 6 points: %+v, want 4.67 at band 7", got)
	}
	if score.Band != nil || score.PendingManual != 1 {
		t.Errorf("with one essay ungraded the paper has band %v (%d pending); it has none yet", score.Band, score.PendingManual)
	}

	score, err = store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.manual, 2.17) || !is(got.band, 6.5) || !slices.Equal(got.criteria, []float64{6, 6.5, 6.5, 6}) {
		t.Errorf("criteria with mean 6.25 of 3 points: %+v, want 2.17 at band 6.5", got)
	}
	if !is(score.Band, 7) || score.Earned != 6.84 || score.PendingManual != 0 {
		t.Errorf("score %+v: (7 × 6 + 6.5 × 3) ÷ 9 = 6.83 rounds to band 7.0, earned 4.67 + 2.17", score)
	}

	score, err = store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(3, 3, 3, 3.5)}})
	if err != nil {
		t.Fatal(err)
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.manual, 1) || !is(got.band, 3) || !slices.Equal(got.criteria, []float64{3, 3, 3, 3.5}) {
		t.Errorf("a regrade with mean 3.125: %+v, want its criteria to replace the old ones at band 3.0", got)
	}
	if !is(score.Band, 5.5) {
		t.Errorf("paper band %v: (7 × 6 + 3 × 3) ÷ 9 = 5.67 rounds to 5.5", score.Band)
	}
}

func TestAGradeThatDoesNotFitItsEssayIsRefusedAndSavesNothing(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	grader := domain.Request{ActorID: p.admin}
	if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(6, 6, 6, 6)}}); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		items []domain.GradeItem
		shape domain.GradeShape
		ids   []string
	}{
		"three of four criteria":    {[]domain.GradeItem{{QuestionID: p.plain, Band: pts(8)}, {QuestionID: p.rubric, Criteria: p.bands(7, 7, 7)}}, domain.CriteriaIncomplete, []string{p.rubric}},
		"another essay's criterion": {[]domain.GradeItem{{QuestionID: p.rubric, Criteria: append(p.bands(7, 7, 7), domain.CriterionBand{CriterionID: uuid.NewString(), Band: 7})}}, domain.CriteriaIncomplete, []string{p.rubric}},
		"points on an essay":        {[]domain.GradeItem{{QuestionID: p.plain, Points: pts(5)}}, domain.NeedsBand, []string{p.plain}},
		"a band on a rubric":        {[]domain.GradeItem{{QuestionID: p.rubric, Band: pts(7)}}, domain.CriteriaIncomplete, []string{p.rubric}},
	} {
		_, err := store.Grade(ctx, everyone, p.attempt, grader, tc.items)
		var shaped *domain.GradeShapeError
		if !errors.As(err, &shaped) || shaped.Shape != tc.shape || !slices.Equal(shaped.QuestionIDs, tc.ids) {
			t.Errorf("%s: %v, want %s naming %v", name, err, tc.shape, tc.ids)
		}
	}
	if got := markOf(t, pool, p.attempt, p.rubric); !is(got.band, 6) || !slices.Equal(got.criteria, []float64{6, 6, 6, 6}) {
		t.Errorf("a refused grade changed the rubric essay: %+v", got)
	}
	if got := markOf(t, pool, p.attempt, p.plain); got.manual != nil || got.band != nil {
		t.Errorf("a refused grade marked the plain essay: %+v", got)
	}
}

func TestTheShapeOfAGradeIsJudgedOnlyOnceTheAttemptIsFound(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	if _, err := store.Grade(ctx, everyone, uuid.NewString(), domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay, Band: pts(6)}}); !errors.Is(err, domain.ErrPaperNotFound) {
		t.Errorf("a band on a missing attempt: %v, want not found", err)
	}
	live := seedPaper(t, pool, "in_progress")
	if _, err := store.Grade(ctx, everyone, live.attempt, domain.Request{ActorID: live.admin}, []domain.GradeItem{{QuestionID: live.essay}}); !errors.Is(err, domain.ErrPaperInProgress) {
		t.Errorf("no points on a live attempt: %v, want in progress", err)
	}
	var shaped *domain.GradeShapeError
	_, err := store.Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay}, {QuestionID: p.choice, Points: pts(1), Band: pts(6)}})
	if !errors.As(err, &shaped) || shaped.Shape != domain.BandNotAllowed || !slices.Equal(shaped.QuestionIDs, []string{p.choice}) {
		t.Errorf("a band beside missing points: %v, want the band refused first", err)
	}
}

func TestAMixedPaperHasNoBand(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.test_version_questions SET type = 'essay' WHERE id = $1::uuid`, p.essay); err != nil {
		t.Fatal(err)
	}
	score, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{{QuestionID: p.essay, Band: pts(9)}})
	if err != nil {
		t.Fatal(err)
	}
	if score.Band != nil || score.Earned != 10 {
		t.Errorf("an essay at band 9 beside a choice question: %+v, want 10 points and no band", score)
	}
}

func TestAGradeIsAuditedWithItsScoresBandsAndCriteria(t *testing.T) {
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	store := repositories.NewReviews(db.NewContext(pool))
	ctx := context.Background()
	grader := domain.Request{ActorID: p.admin, IP: "203.0.113.9", UserAgent: "grader-test"}
	if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{{QuestionID: p.rubric, Criteria: p.bands(6, 6, 6, 6)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Grade(ctx, everyone, p.attempt, grader, []domain.GradeItem{
		{QuestionID: p.rubric, Criteria: p.bands(7, 7, 7, 8), Comment: comment("RIENG-TU")},
		{QuestionID: p.plain, Band: pts(6.5)},
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM app.audit_log WHERE entity_id = $1::uuid`, p.attempt)
	})

	var raw []byte
	var actor, ip, agent string
	if err := pool.QueryRow(ctx, `
		SELECT diff, actor_user_id::text, host(ip), user_agent FROM app.audit_log
		 WHERE action = 'attempt.graded' AND entity = 'attempt' AND entity_id = $1::uuid
		 ORDER BY id DESC LIMIT 1`, p.attempt).Scan(&raw, &actor, &ip, &agent); err != nil {
		t.Fatal(err)
	}
	if n := count(t, pool, `SELECT count(*) FROM app.audit_log WHERE action = 'attempt.graded' AND entity_id = $1::uuid`, p.attempt); n != 2 {
		t.Errorf("%d grade entries, want one per grade", n)
	}
	if actor != p.admin || ip != "203.0.113.9" || agent != "grader-test" {
		t.Errorf("entry by %s from %s %s", actor, ip, agent)
	}
	type change[T any] struct{ Old, New T }
	var diff struct {
		Questions map[string]struct {
			ManualScore change[*float64]            `json:"manual_score"`
			Band        *change[*float64]           `json:"band"`
			Criteria    *change[map[string]float64] `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(raw, &diff); err != nil {
		t.Fatal(err)
	}
	rubric, plain := diff.Questions[p.rubric], diff.Questions[p.plain]
	if !is(rubric.ManualScore.Old, 2) || !is(rubric.ManualScore.New, 2.5) || !is(rubric.Band.Old, 6) || !is(rubric.Band.New, 7.5) {
		t.Errorf("rubric essay %+v, want 2 → 2.5 and band 6 → 7.5", rubric)
	}
	if rubric.Criteria.Old[p.criteria[3]] != 6 || rubric.Criteria.New[p.criteria[3]] != 8 || len(rubric.Criteria.New) != 4 {
		t.Errorf("rubric criteria %+v", rubric.Criteria)
	}
	if plain.ManualScore.Old != nil || !is(plain.ManualScore.New, 4.33) || plain.Band.Old != nil || !is(plain.Band.New, 6.5) || len(plain.Criteria.New) != 0 {
		t.Errorf("plain essay %+v, want nothing → 4.33 at band 6.5", plain)
	}
	if strings.Contains(string(raw), "RIENG-TU") {
		t.Error("the comment reached the audit diff")
	}
}
