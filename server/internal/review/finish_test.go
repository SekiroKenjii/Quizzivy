package review_test

import (
	"context"
	"errors"
	"testing"

	"quizzivy/internal/attempts"
	"quizzivy/internal/review"
)

func TestFinishingWithAnUngradedShortAnswerIsRefused(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := review.NewStore(pool)
	ctx := context.Background()

	if _, err := store.Finish(ctx, p.attempt); !errors.Is(err, review.ErrIncomplete) {
		t.Fatalf("finish with the essay unread: %v, want ErrIncomplete", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" {
		t.Errorf("status %s after a refused finish, want submitted", status)
	}
}

func TestFinishingRecomputesTheScoreAndIsReEnterable(t *testing.T) {
	pool := newPool(t)
	p := seedPaper(t, pool, "submitted")
	store := review.NewStore(pool)
	ctx := context.Background()

	if _, err := store.Grade(ctx, p.attempt, p.admin, []review.Item{{QuestionID: p.essay, Points: 2.5}}); err != nil {
		t.Fatal(err)
	}
	graded, err := store.Finish(ctx, p.attempt)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if graded.Status != attempts.Graded || graded.GradedAt == nil {
		t.Errorf("finished attempt %+v, want graded with a timestamp", graded)
	}
	var earned float64
	if err := pool.QueryRow(ctx, `SELECT score_earned FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&earned); err != nil {
		t.Fatal(err)
	}
	if earned != 7.5 {
		t.Errorf("score_earned %v, want 7.5 = SUM(final_score)", earned)
	}

	// Second thoughts: the mark changes and so does the total.
	if _, err := store.Grade(ctx, p.attempt, p.admin, []review.Item{{QuestionID: p.essay, Points: 5}}); err != nil {
		t.Fatalf("regrade after finish: %v", err)
	}
	if _, err := store.Finish(ctx, p.attempt); err != nil {
		t.Fatalf("finish again: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT score_earned FROM app.attempts WHERE id = $1::uuid`, p.attempt).Scan(&earned); err != nil {
		t.Fatal(err)
	}
	if earned != 10 {
		t.Errorf("score_earned %v after regrading, want 10", earned)
	}
}
