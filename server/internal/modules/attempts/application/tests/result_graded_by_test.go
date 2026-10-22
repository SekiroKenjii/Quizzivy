//go:build integration

package application_test

import (
	"context"
	"quizzivy/internal/core/adapters"
	"quizzivy/internal/modules/attempts/application"
	"quizzivy/internal/modules/attempts/application/query"
	"quizzivy/internal/modules/attempts/domain"
	"quizzivy/internal/modules/attempts/repositories"
	"quizzivy/internal/platform/db"
	"quizzivy/internal/shared/retention"
	"testing"
)

// essayResult is an essay paper whose two essays both wait for a grader, and
// the student's score on it under showScore.
func essayResult(t *testing.T) (essayPaper, func() *domain.Score) {
	t.Helper()
	pool := newPool(t)
	p := seedEssayPaper(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE app.attempt_answers SET requires_manual = true, auto_score = NULL WHERE attempt_id = $1::uuid`, p.attempt); err != nil {
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

func grade(t *testing.T, attempt, grader string, items ...domain.GradeItem) {
	t.Helper()
	if _, err := repositories.NewReviews(db.NewContext(newPool(t))).Grade(context.Background(), everyone, attempt, domain.Request{ActorID: grader}, items); err != nil {
		t.Fatal(err)
	}
}

// gradedEssayPaper is essayResult with both essays graded by its Admin.
func gradedEssayPaper(t *testing.T) (essayPaper, func() *domain.Score) {
	t.Helper()
	p, score := essayResult(t)
	grade(t, p.attempt, p.admin, domain.GradeItem{QuestionID: p.plain, Band: pts(7)}, domain.GradeItem{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)})
	return p, score
}

func TestTheGraderIsNamedOnlyOnceNothingWaits(t *testing.T) {
	p, score := essayResult(t)
	grade(t, p.attempt, p.admin, domain.GradeItem{QuestionID: p.plain, Band: pts(7)})
	if got := score(); got == nil || got.PendingManual != 1 || got.GradedBy != nil {
		t.Fatalf("one essay still waiting: score %+v, want one pending and no grader", got)
	}
	grade(t, p.attempt, p.admin, domain.GradeItem{QuestionID: p.rubric, Criteria: p.bands(6, 6.5, 6.5, 6)})
	if got := score(); got == nil || got.GradedBy == nil || *got.GradedBy != "Giáo viên" {
		t.Fatalf("both essays graded: score %+v, want it graded by Giáo viên", got)
	}
}

// overriddenPaper is seedPaper's choice question with its auto mark
// overridden by the Admin, read back as the student under showScore. With
// essay false the paper has no answer that waits for a grader.
func overriddenPaper(t *testing.T, essay bool) *domain.Score {
	t.Helper()
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	ctx := context.Background()
	items := []domain.GradeItem{{QuestionID: p.choice, Points: pts(4)}}
	if essay {
		items = append(items, domain.GradeItem{QuestionID: p.essay, Points: pts(3)})
	} else if _, err := pool.Exec(ctx, `DELETE FROM app.attempt_answers WHERE attempt_id = $1::uuid AND question_id = $2::uuid`, p.attempt, p.essay); err != nil {
		t.Fatal(err)
	}
	grade(t, p.attempt, p.admin, items...)
	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	setReview(t, pool, world{assignment: assignment}, true, false, false)
	result, err := newService(t, pool).Queries.Result.Handle(ctx, query.Result{AttemptID: p.attempt, StudentID: p.student})
	if err != nil {
		t.Fatal(err)
	}
	return result.Score
}

func TestOverridingAnAutoMarkGradesNoPaper(t *testing.T) {
	if got := overriddenPaper(t, false); got == nil || got.GradedBy != nil {
		t.Fatalf("score %+v, want no grader on an auto-marked paper", got)
	}
}

func TestAnOverrideBesideAHandGradedEssayNamesItsGrader(t *testing.T) {
	if got := overriddenPaper(t, true); got == nil || got.GradedBy == nil || *got.GradedBy != "Giáo viên" {
		t.Fatalf("score %+v, want it graded by Giáo viên", got)
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
	pool := newPool(t)
	ctx := context.Background()
	var gone string
	if err := pool.QueryRow(ctx, `INSERT INTO app.users (email, full_name, role_id)
		VALUES ('gone-'||gen_random_uuid()||'@example.com', 'Thầy Cũ', (SELECT id FROM app.roles WHERE builtin_key = 'student'))
		RETURNING id::text`).Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE app.attempt_answers SET graded_by = $3::uuid WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		p.attempt, p.plain, gone); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM app.users WHERE id = $1::uuid`, gone); err != nil {
		t.Fatal(err)
	}
	if got := score(); got == nil || got.GradedBy != nil {
		t.Fatalf("score %+v, want no grader named once one grader's account is deleted", got)
	}
}

func TestStaffScoresNeverNameTheGrader(t *testing.T) {
	p, score := gradedEssayPaper(t)
	if got := score(); got == nil || got.GradedBy == nil {
		t.Fatalf("score %+v, want the student's result to name the grader", got)
	}
	pool := newPool(t)
	ctx := context.Background()
	dbx := db.NewContext(pool)
	svc := application.New(nil, repositories.NewReviews(dbx), repositories.NewPostgres(dbx, adapters.AttemptStartGuard{}))
	review, err := svc.Queries.Review.Handle(ctx, query.Review{Scope: everyone, AttemptID: p.attempt})
	if err != nil {
		t.Fatal(err)
	}
	if review.Score.GradedBy != nil {
		t.Errorf("the review's score names %q", *review.Score.GradedBy)
	}
	graded, err := repositories.NewReviews(db.NewContext(pool)).Grade(ctx, everyone, p.attempt, domain.Request{ActorID: p.admin},
		[]domain.GradeItem{{QuestionID: p.plain, Band: pts(7)}})
	if err != nil {
		t.Fatal(err)
	}
	if graded.GradedBy != nil {
		t.Errorf("the grade response names %q", *graded.GradedBy)
	}
	var assignment string
	if err := pool.QueryRow(ctx, `SELECT assignment_id::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&assignment); err != nil {
		t.Fatal(err)
	}
	monitor, err := svc.Queries.Monitor.Handle(ctx, query.Monitor{Scope: everyone, AssignmentID: assignment})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range monitor.Rows {
		if row.Score != nil && row.Score.GradedBy != nil {
			t.Errorf("the monitor row for %s names %q", row.StudentID, *row.Score.GradedBy)
		}
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

func TestAnAnswerRetentionRemovedDoesNotKeepTheGraderOut(t *testing.T) {
	p, score := essayResult(t)
	if _, err := newPool(t).Exec(context.Background(), `UPDATE app.attempt_answers SET payload = '`+retention.RemovedAnswer+`'::jsonb WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		p.attempt, p.rubric); err != nil {
		t.Fatal(err)
	}
	grade(t, p.attempt, p.admin, domain.GradeItem{QuestionID: p.plain, Band: pts(7)})
	if got := score(); got == nil || got.GradedBy == nil || *got.GradedBy != "Giáo viên" {
		t.Fatalf("score %+v, want it graded by Giáo viên", got)
	}
}
