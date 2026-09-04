package review_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/review"
)

func comment(s string) *string { return &s }

// final_score is coalesce(manual, auto): the essay's mark fills its gap, and a
// manual mark on the choice question overrides the machine's.
func TestTheScoreUsesTheManualMarkWherePresentAndTheAutoScoreOtherwise(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := review.NewStore(pool)
	ctx := context.Background()

	score, err := store.Grade(ctx, p.attempt, p.admin, []review.Item{
		{QuestionID: p.essay, Points: 4, Comment: comment("Ý tốt, câu thứ hai thiếu động từ chính.")},
	})
	if err != nil {
		t.Fatalf("grade: %v", err)
	}
	if score.Earned != 9 || score.Total != 10 || score.PendingManual != 0 {
		t.Errorf("after the essay: %+v, want 9/10 with nothing pending", score)
	}

	score, err = store.Grade(ctx, p.attempt, p.admin, []review.Item{{QuestionID: p.choice, Points: 3}})
	if err != nil {
		t.Fatalf("regrade the choice: %v", err)
	}
	if score.Earned != 7 {
		t.Errorf("after overriding the machine: earned %v, want 7 (3 manual + 4 manual)", score.Earned)
	}

	var gradedBy string
	var stored float64
	if err := pool.QueryRow(ctx, `SELECT graded_by::text, score_earned FROM app.attempt_answers a
		JOIN app.attempts at ON at.id = a.attempt_id WHERE a.attempt_id = $1::uuid AND a.question_id = $2::uuid`,
		p.attempt, p.essay).Scan(&gradedBy, &stored); err != nil {
		t.Fatal(err)
	}
	if gradedBy != p.admin {
		t.Errorf("graded_by %s, want the teacher", gradedBy)
	}
	if stored != 7 {
		t.Errorf("attempts.score_earned %v, want 7 folded back onto the attempt", stored)
	}

	rv, err := store.Get(ctx, p.attempt)
	if err != nil {
		t.Fatal(err)
	}
	if rv.Answers[p.essay].GraderComment == nil || rv.Score.Earned != 7 {
		t.Errorf("review reads %+v / %+v", rv.Answers[p.essay], rv.Score)
	}
}

func TestPointsAboveTheQuestionsCeilingAreRefused(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := review.NewStore(pool)

	_, err := store.Grade(context.Background(), p.attempt, p.admin, []review.Item{{QuestionID: p.essay, Points: 5.5}})
	var invalid *review.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Items) != 1 || invalid.Items[0].Reason != "above_ceiling" {
		t.Fatalf("got %v, want one above_ceiling item", err)
	}
	var manual *float64
	if err := pool.QueryRow(context.Background(), `SELECT manual_score FROM app.attempt_answers
		WHERE attempt_id = $1::uuid AND question_id = $2::uuid`, p.attempt, p.essay).Scan(&manual); err != nil {
		t.Fatal(err)
	}
	if manual != nil {
		t.Error("a refused mark was stored")
	}
}

func TestAQuestionOffThePaperOrNeverAnsweredCannotBeMarked(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := review.NewStore(pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DELETE FROM app.attempt_answers WHERE attempt_id = $1::uuid AND question_id = $2::uuid`,
		p.attempt, p.essay); err != nil {
		t.Fatal(err)
	}

	_, err := store.Grade(ctx, p.attempt, p.admin, []review.Item{
		{QuestionID: p.essay, Points: 1},
		{QuestionID: "01935000-0000-7000-8000-00000000dead", Points: 1},
	})
	var invalid *review.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Items) != 2 {
		t.Fatalf("got %v, want two refused items", err)
	}
	reasons := map[string]string{}
	for _, it := range invalid.Items {
		reasons[it.QuestionID] = it.Reason
	}
	if reasons[p.essay] != "unanswered" || reasons["01935000-0000-7000-8000-00000000dead"] != "not_on_paper" {
		t.Errorf("reasons %v", reasons)
	}
}

func TestGradingWaitsForTheStudentAndSkipsAVoidedAttempt(t *testing.T) {
	pool := newPool(t)
	store := review.NewStore(pool)
	ctx := context.Background()

	live := seedPaper(t, pool, "in_progress")
	if _, err := store.Grade(ctx, live.attempt, live.admin, []review.Item{{QuestionID: live.essay, Points: 1}}); !errors.Is(err, review.ErrInProgress) {
		t.Errorf("in progress: %v, want ErrInProgress", err)
	}
	voided := seedPaper(t, pool, "voided")
	if _, err := store.Grade(ctx, voided.attempt, voided.admin, []review.Item{{QuestionID: voided.essay, Points: 1}}); !errors.Is(err, review.ErrVoided) {
		t.Errorf("voided: %v, want ErrVoided", err)
	}
	if _, err := store.Get(ctx, "01935000-0000-7000-8000-00000000dead"); !errors.Is(err, review.ErrNotFound) {
		t.Errorf("unknown: %v, want ErrNotFound", err)
	}
}

func TestTheReviewCarriesTheKeyThePaperAndTheAnswers(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")

	rv, err := review.NewStore(pool).Get(context.Background(), p.attempt)
	if err != nil {
		t.Fatal(err)
	}
	if rv.TestTitle != "Unit 5" || rv.MaxAttempts != 2 || len(rv.Questions) != 2 {
		t.Errorf("review header %q %d %d", rv.TestTitle, rv.MaxAttempts, len(rv.Questions))
	}
	if rv.Questions[0].ID != p.choice || rv.Questions[1].ID != p.essay {
		t.Error("questions are not in the version's order")
	}
	if rv.Questions[1].SampleAnswer == nil || *rv.Questions[1].SampleAnswer != "I get up at six." {
		t.Error("the sample answer is the grader's and should be here")
	}
	correct := 0
	for _, o := range rv.Questions[0].Options {
		if o.IsCorrect {
			correct++
		}
	}
	if correct != 1 {
		t.Errorf("%d correct options, want 1", correct)
	}
	if rv.Score.Earned != 5 || rv.Score.PendingManual != 1 {
		t.Errorf("score %+v, want 5 earned with one pending", rv.Score)
	}
}
