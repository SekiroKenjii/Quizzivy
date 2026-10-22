//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"testing"
)

// gradedEssayPaper is an essay paper with both essays graded by its Admin,
// read back as the student under showScore.
func gradedEssayPaper(t *testing.T) (essayPaper, func() *domain.Score) {
	t.Helper()
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	ctx := context.Background()
	if _, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin}, []domain.GradeItem{
		{QuestionID: p.plain, Band: pts(7)},
		{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)},
	}); err != nil {
		t.Fatal(err)
	}
	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	setReview(t, pool, world{assignment: assignment}, true, false, false)
	svc := newService(t, pool)
	return p, func() *domain.Score {
		t.Helper()
		result, err := svc.Queries.Result.Handle(ctx, query.Result{AttemptID: p.attempt, StudentID: p.student})
		if err != nil {
			t.Fatal(err)
		}
		return result.Score
	}
}

func TestOneGraderIsNamedOnTheStudentsResult(t *testing.T) {
	_, score := gradedEssayPaper(t)
	if got := score(); got == nil || got.GradedBy == nil || *got.GradedBy != "Giáo viên" {
		t.Fatalf("score %+v, want it graded by Giáo viên", got)
	}
}

func TestTheGraderPrefersTheDisplayName(t *testing.T) {
	p, score := gradedEssayPaper(t)
	if _, err := newPool(t).Exec(context.Background(), `UPDATE app.users SET display_name = 'Cô Thương' WHERE id = $1::uuid`, p.admin); err != nil {
		t.Fatal(err)
	}
	if got := score(); got == nil || got.GradedBy == nil || *got.GradedBy != "Cô Thương" {
		t.Fatalf("score %+v, want it graded by Cô Thương", got)
	}
}

func TestTwoGradersNameNobody(t *testing.T) {
	p, score := gradedEssayPaper(t)
	pool := newPool(t)
	ctx := context.Background()
	var other string
	if err := pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ('grader-'||gen_random_uuid()||'@example.com', 'Thầy Hai', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
		RETURNING id::text`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanUp(t, pool, `DELETE FROM app.users WHERE id = $1::uuid`, other) })
	if _, err := pool.Exec(ctx, `UPDATE app.attempt_answers SET graded_by = $3::uuid WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		p.attempt, p.plain, other); err != nil {
		t.Fatal(err)
	}
	if got := score(); got == nil || got.GradedBy != nil {
		t.Fatalf("score %+v, want no grader named", got)
	}
}

func TestAGraderWhoseAccountIsGoneNamesNobody(t *testing.T) {
	p, score := gradedEssayPaper(t)
	if _, err := newPool(t).Exec(context.Background(), `UPDATE app.attempt_answers SET graded_by = NULL WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		p.attempt, p.plain); err != nil {
		t.Fatal(err)
	}
	if got := score(); got == nil || got.GradedBy != nil {
		t.Fatalf("score %+v, want no grader named", got)
	}
}

func TestAnAnonymisedGraderNamesNobody(t *testing.T) {
	p, score := gradedEssayPaper(t)
	if _, err := newPool(t).Exec(context.Background(),
		`UPDATE app.users u SET email = 'anonymous-' || u.id::text || '@anonymous.invalid' WHERE u.id = $1::uuid`, p.admin); err != nil {
		t.Fatal(err)
	}
	if got := score(); got == nil || got.GradedBy != nil {
		t.Fatalf("score %+v, want no grader named", got)
	}
}

func TestAWithheldScoreNamesNobody(t *testing.T) {
	p, score := gradedEssayPaper(t)
	pool := newPool(t)
	var assignment string
	if err := pool.QueryRow(context.Background(), `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	setReview(t, pool, world{assignment: assignment}, false, true, true)
	if got := score(); got != nil {
		t.Fatalf("score %+v while withheld, want none and so no grader", got)
	}
}

func TestAPaperNobodyGradedByHandNamesNobody(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM app.attempt_answers WHERE attempt_id = $1::uuid AND question_id = $2::uuid`, p.attempt, p.essay); err != nil {
		t.Fatal(err)
	}
	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	setReview(t, pool, world{assignment: assignment}, true, false, false)
	result, err := newService(t, pool).Queries.Result.Handle(ctx, query.Result{AttemptID: p.attempt, StudentID: p.student})
	if err != nil {
		t.Fatal(err)
	}
	if result.Score == nil || result.Score.GradedBy != nil {
		t.Fatalf("score %+v, want a score and no grader", result.Score)
	}
}
